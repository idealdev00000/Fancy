package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/echovault/echovault/src/memberlist"
	"github.com/echovault/echovault/src/modules/acl"
	"github.com/echovault/echovault/src/modules/pubsub"
	"github.com/echovault/echovault/src/raft"
	"github.com/echovault/echovault/src/server/aof"
	"github.com/echovault/echovault/src/server/snapshot"
	"github.com/echovault/echovault/src/utils"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	Config utils.Config

	ConnID atomic.Uint64

	store           map[string]interface{}
	keyLocks        map[string]*sync.RWMutex
	keyCreationLock *sync.Mutex

	commands []utils.Command

	raft       *raft.Raft
	memberList *memberlist.MemberList

	CancelCh *chan os.Signal

	ACL    *acl.ACL
	PubSub *pubsub.PubSub

	SnapshotInProgress         atomic.Bool
	RewriteAOFInProgress       atomic.Bool
	StateCopyInProgress        atomic.Bool
	StateMutationInProgress    atomic.Bool
	LatestSnapshotMilliseconds atomic.Int64 // Unix epoch in milliseconds
	SnapshotEngine             *snapshot.Engine
	AOFEngine                  *aof.Engine
}

func (server *Server) StartTCP(ctx context.Context) {
	conf := server.Config

	listenConfig := net.ListenConfig{
		KeepAlive: 200 * time.Millisecond,
	}

	listener, err := listenConfig.Listen(ctx, "tcp", fmt.Sprintf("%s:%d", conf.BindAddr, conf.Port))

	if err != nil {
		log.Fatal(err)
	}

	if !conf.TLS {
		// TCP
		fmt.Printf("Starting TCP server at Address %s, Port %d...\n", conf.BindAddr, conf.Port)
	}

	if conf.TLS {
		// TLS
		fmt.Printf("Starting TLS server at Address %s, Port %d...\n", conf.BindAddr, conf.Port)
		cer, err := tls.LoadX509KeyPair(conf.Cert, conf.Key)
		if err != nil {
			log.Fatal(err)
		}

		listener = tls.NewListener(listener, &tls.Config{
			Certificates: []tls.Certificate{cer},
		})
	}

	// Listen to connection
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Could not establish connection")
			continue
		}
		// Read loop for connection
		go server.handleConnection(ctx, conn)
	}
}

func (server *Server) handleConnection(ctx context.Context, conn net.Conn) {
	server.ACL.RegisterConnection(&conn)

	w, r := io.Writer(conn), io.Reader(conn)

	cid := server.ConnID.Add(1)
	ctx = context.WithValue(ctx, utils.ContextConnID("ConnectionID"),
		fmt.Sprintf("%s-%d", ctx.Value(utils.ContextServerID("ServerID")), cid))

	for {
		message, err := utils.ReadMessage(r)

		if err != nil && errors.Is(err, io.EOF) {
			// Connection closed
			break
		}

		var netErr net.Error

		if err != nil && errors.As(err, &netErr) && netErr.Timeout() {
			// Connection timeout
			log.Println(err)
			break
		}

		if err != nil && errors.Is(err, tls.RecordHeaderError{}) {
			// TLS verification error
			log.Println(err)
			break
		}

		if err != nil {
			log.Println(err)
			break
		}

		res, err := server.handleCommand(ctx, message, &conn, false)

		if err != nil {
			if _, err = w.Write([]byte(fmt.Sprintf("-Error %s", err.Error()))); err != nil {
				log.Println(err)
			}
			continue
		}

		if _, err = w.Write(res); err != nil {
			log.Println(err)
		}
	}

	if err := conn.Close(); err != nil {
		log.Println(err)
	}
}

func (server *Server) Start(ctx context.Context) {
	conf := server.Config

	server.store = make(map[string]interface{})
	server.keyLocks = make(map[string]*sync.RWMutex)
	server.keyCreationLock = &sync.Mutex{}

	server.LoadModules(ctx)

	if conf.TLS && (len(conf.Key) <= 0 || len(conf.Cert) <= 0) {
		fmt.Println("Must provide key and certificate file paths for TLS mode.")
		return
	}

	if server.IsInCluster() {
		// Initialise raft and memberlist
		server.raft = raft.NewRaft(raft.Opts{
			Config:     conf,
			Server:     server,
			GetCommand: server.getCommand,
		})
		server.memberList = memberlist.NewMemberList(memberlist.MemberlistOpts{
			Config:           conf,
			HasJoinedCluster: server.raft.HasJoinedCluster,
			AddVoter:         server.raft.AddVoter,
			RemoveRaftServer: server.raft.RemoveServer,
			IsRaftLeader:     server.raft.IsRaftLeader,
			ApplyMutate:      server.raftApply,
		})
		server.raft.RaftInit(ctx)
		server.memberList.MemberListInit(ctx)
	} else {
		// Initialize and start standalone snapshot engine
		server.SnapshotEngine = snapshot.NewSnapshotEngine(snapshot.Opts{
			Config:                        conf,
			StartSnapshot:                 server.StartSnapshot,
			FinishSnapshot:                server.FinishSnapshot,
			GetState:                      server.GetState,
			SetLatestSnapshotMilliseconds: server.SetLatestSnapshot,
			GetLatestSnapshotMilliseconds: server.GetLatestSnapshot,
			CreateKeyAndLock:              server.CreateKeyAndLock,
			KeyUnlock:                     server.KeyUnlock,
			SetValue:                      server.SetValue,
		})
		if conf.RestoreSnapshot {
			err := server.SnapshotEngine.Restore(ctx)
			if err != nil {
				log.Println(err)
			}
		}
		server.SnapshotEngine.Start(ctx)

		// Initialize standalone AOF engine
		server.AOFEngine = aof.NewAOFEngine(aof.Opts{
			Config:           conf,
			GetState:         server.GetState,
			StartRewriteAOF:  server.StartRewriteAOF,
			FinishRewriteAOF: server.FinishRewriteAOF,
			CreateKeyAndLock: server.CreateKeyAndLock,
			KeyUnlock:        server.KeyUnlock,
			SetValue:         server.SetValue,
			HandleCommand:    server.handleCommand,
		})
		if conf.RestoreAOF && !conf.RestoreSnapshot {
			err := server.AOFEngine.Restore(ctx)
			if err != nil {
				log.Println(err)
			}
		}
		server.AOFEngine.Start(ctx)
	}

	server.StartTCP(ctx)
}

func (server *Server) TakeSnapshot() error {
	if server.SnapshotInProgress.Load() {
		return errors.New("snapshot already in progress")
	}

	go func() {
		if server.IsInCluster() {
			// Handle snapshot in cluster mode
			if err := server.raft.TakeSnapshot(); err != nil {
				log.Println(err)
			}
			return
		}
		// Handle snapshot in standalone mode
		if err := server.SnapshotEngine.TakeSnapshot(); err != nil {
			log.Println(err)
		}
	}()

	return nil
}

func (server *Server) StartSnapshot() {
	server.SnapshotInProgress.Store(true)
}

func (server *Server) FinishSnapshot() {
	server.SnapshotInProgress.Store(false)
}

func (server *Server) SetLatestSnapshot(msec int64) {
	server.LatestSnapshotMilliseconds.Store(msec)
}

func (server *Server) GetLatestSnapshot() int64 {
	return server.LatestSnapshotMilliseconds.Load()
}

func (server *Server) StartRewriteAOF() {
	server.RewriteAOFInProgress.Store(true)
}

func (server *Server) FinishRewriteAOF() {
	server.RewriteAOFInProgress.Store(false)
}

func (server *Server) RewriteAOF() error {
	if server.RewriteAOFInProgress.Load() {
		return errors.New("aof rewrite in progress")
	}
	go func() {
		if err := server.AOFEngine.RewriteLog(); err != nil {
			log.Println(err)
		}
	}()
	return nil
}

func (server *Server) ShutDown(ctx context.Context) {
	if server.IsInCluster() {
		server.raft.RaftShutdown(ctx)
		server.memberList.MemberListShutdown(ctx)
	}
}
