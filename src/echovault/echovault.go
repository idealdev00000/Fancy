// Copyright 2024 Kelvin Clement Mwinuka
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package echovault

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/echovault/echovault/src/aof"
	"github.com/echovault/echovault/src/eviction"
	"github.com/echovault/echovault/src/memberlist"
	"github.com/echovault/echovault/src/raft"
	"github.com/echovault/echovault/src/snapshot"
	"github.com/echovault/echovault/src/utils"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type EchoVault struct {
	// Config holds the echovault configuration variables.
	Config utils.Config

	// The current index for the latest connection id.
	// This number is incremented everytime there's a new connection and
	// the new number is the new connection's ID.
	ConnID atomic.Uint64

	store           map[string]utils.KeyData // Data store to hold the keys and their associated data, expiry time, etc.
	keyLocks        map[string]*sync.RWMutex // Map to hold all the individual key locks.
	keyCreationLock *sync.Mutex              // The mutex for creating a new key. Only one goroutine should be able to create a key at a time.

	// Holds all the keys that are currently associated with an expiry.
	keysWithExpiry struct {
		rwMutex sync.RWMutex // Mutex as only one process should be able to update this list at a time.
		keys    []string     // string slice of the volatile keys
	}
	// LFU cache used when eviction policy is allkeys-lfu or volatile-lfu
	lfuCache struct {
		mutex sync.Mutex        // Mutex as only one goroutine can edit the LFU cache at a time.
		cache eviction.CacheLFU // LFU cache represented by a min head.
	}
	// LRU cache used when eviction policy is allkeys-lru or volatile-lru
	lruCache struct {
		mutex sync.Mutex        // Mutex as only one goroutine can edit the LRU at a time.
		cache eviction.CacheLRU // LRU cache represented by a max head.
	}

	// Holds the list of all commands supported by the echovault.
	Commands []utils.Command

	raft       *raft.Raft             // The raft replication layer for the echovault.
	memberList *memberlist.MemberList // The memberlist layer for the echovault.

	Context context.Context

	ACL    utils.ACL
	PubSub utils.PubSub

	SnapshotInProgress         atomic.Bool      // Atomic boolean that's true when actively taking a snapshot.
	RewriteAOFInProgress       atomic.Bool      // Atomic boolean that's true when actively rewriting AOF file is in progress.
	StateCopyInProgress        atomic.Bool      // Atomic boolean that's true when actively copying state for snapshotting or preamble generation.
	StateMutationInProgress    atomic.Bool      // Atomic boolean that is set to true when state mutation is in progress.
	LatestSnapshotMilliseconds atomic.Int64     // Unix epoch in milliseconds
	SnapshotEngine             *snapshot.Engine // Snapshot engine for standalone mode
	AOFEngine                  *aof.Engine      // AOF engine for standalone mode
}

func WithContext(ctx context.Context) func(echovault *EchoVault) {
	return func(echovault *EchoVault) {
		echovault.Context = ctx
	}
}

func WithConfig(config utils.Config) func(echovault *EchoVault) {
	return func(echovault *EchoVault) {
		echovault.Config = config
	}
}

func WithACL(acl utils.ACL) func(echovault *EchoVault) {
	return func(echovault *EchoVault) {
		echovault.ACL = acl
	}
}

func WithPubSub(pubsub utils.PubSub) func(echovault *EchoVault) {
	return func(echovault *EchoVault) {
		echovault.PubSub = pubsub
	}
}

func WithCommands(commands []utils.Command) func(echovault *EchoVault) {
	return func(echovault *EchoVault) {
		echovault.Commands = commands
	}
}

func NewEchoVault(options ...func(echovault *EchoVault)) *EchoVault {
	echovault := &EchoVault{
		Context:         context.Background(),
		Commands:        make([]utils.Command, 0),
		store:           make(map[string]utils.KeyData),
		keyLocks:        make(map[string]*sync.RWMutex),
		keyCreationLock: &sync.Mutex{},
	}

	for _, option := range options {
		option(echovault)
	}

	if echovault.isInCluster() {
		echovault.raft = raft.NewRaft(raft.Opts{
			Config:     echovault.Config,
			EchoVault:  echovault,
			GetCommand: echovault.getCommand,
			DeleteKey:  echovault.DeleteKey,
		})
		echovault.memberList = memberlist.NewMemberList(memberlist.Opts{
			Config:           echovault.Config,
			HasJoinedCluster: echovault.raft.HasJoinedCluster,
			AddVoter:         echovault.raft.AddVoter,
			RemoveRaftServer: echovault.raft.RemoveServer,
			IsRaftLeader:     echovault.raft.IsRaftLeader,
			ApplyMutate:      echovault.raftApplyCommand,
			ApplyDeleteKey:   echovault.raftApplyDeleteKey,
		})
	} else {
		// Set up standalone snapshot engine
		echovault.SnapshotEngine = snapshot.NewSnapshotEngine(
			snapshot.WithDirectory(echovault.Config.DataDir),
			snapshot.WithThreshold(echovault.Config.SnapShotThreshold),
			snapshot.WithInterval(echovault.Config.SnapshotInterval),
			snapshot.WithGetStateFunc(echovault.GetState),
			snapshot.WithStartSnapshotFunc(echovault.StartSnapshot),
			snapshot.WithFinishSnapshotFunc(echovault.FinishSnapshot),
			snapshot.WithSetLatestSnapshotTimeFunc(echovault.SetLatestSnapshot),
			snapshot.WithGetLatestSnapshotTimeFunc(echovault.GetLatestSnapshot),
			snapshot.WithSetKeyDataFunc(func(key string, data utils.KeyData) {
				ctx := context.Background()
				if _, err := echovault.CreateKeyAndLock(ctx, key); err != nil {
					log.Println(err)
				}
				if err := echovault.SetValue(ctx, key, data.Value); err != nil {
					log.Println(err)
				}
				echovault.SetExpiry(ctx, key, data.ExpireAt, false)
				echovault.KeyUnlock(ctx, key)
			}),
		)
		// Set up standalone AOF engine
		echovault.AOFEngine = aof.NewAOFEngine(
			aof.WithDirectory(echovault.Config.DataDir),
			aof.WithStrategy(echovault.Config.AOFSyncStrategy),
			aof.WithStartRewriteFunc(echovault.StartRewriteAOF),
			aof.WithFinishRewriteFunc(echovault.FinishRewriteAOF),
			aof.WithGetStateFunc(echovault.GetState),
			aof.WithSetKeyDataFunc(func(key string, value utils.KeyData) {
				ctx := context.Background()
				if _, err := echovault.CreateKeyAndLock(ctx, key); err != nil {
					log.Println(err)
				}
				if err := echovault.SetValue(ctx, key, value.Value); err != nil {
					log.Println(err)
				}
				echovault.SetExpiry(ctx, key, value.ExpireAt, false)
				echovault.KeyUnlock(ctx, key)
			}),
			aof.WithHandleCommandFunc(func(command []byte) {
				_, err := echovault.handleCommand(context.Background(), command, nil, true)
				if err != nil {
					log.Println(err)
				}
			}),
		)
	}

	// If eviction policy is not noeviction, start a goroutine to evict keys every 100 milliseconds.
	if echovault.Config.EvictionPolicy != utils.NoEviction {
		go func() {
			for {
				<-time.After(echovault.Config.EvictionInterval)
				if err := echovault.evictKeysWithExpiredTTL(context.Background()); err != nil {
					log.Println(err)
				}
			}
		}()
	}

	return echovault
}

func (server *EchoVault) StartTCP(ctx context.Context) {
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
		fmt.Printf("Starting TCP echovault at Address %s, Port %d...\n", conf.BindAddr, conf.Port)
	}

	if conf.TLS || conf.MTLS {
		// TLS
		if conf.TLS {
			fmt.Printf("Starting mTLS echovault at Address %s, Port %d...\n", conf.BindAddr, conf.Port)
		} else {
			fmt.Printf("Starting TLS echovault at Address %s, Port %d...\n", conf.BindAddr, conf.Port)
		}

		var certificates []tls.Certificate
		for _, certKeyPair := range conf.CertKeyPairs {
			c, err := tls.LoadX509KeyPair(certKeyPair[0], certKeyPair[1])
			if err != nil {
				log.Fatal(err)
			}
			certificates = append(certificates, c)
		}

		clientAuth := tls.NoClientCert
		clientCerts := x509.NewCertPool()

		if conf.MTLS {
			clientAuth = tls.RequireAndVerifyClientCert
			for _, c := range conf.ClientCAs {
				ca, err := os.Open(c)
				if err != nil {
					log.Fatal(err)
				}
				certBytes, err := io.ReadAll(ca)
				if err != nil {
					log.Fatal(err)
				}
				if ok := clientCerts.AppendCertsFromPEM(certBytes); !ok {
					log.Fatal(err)
				}
			}
		}

		listener = tls.NewListener(listener, &tls.Config{
			Certificates: certificates,
			ClientAuth:   clientAuth,
			ClientCAs:    clientCerts,
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

func (server *EchoVault) handleConnection(ctx context.Context, conn net.Conn) {
	// If ACL module is loaded, register the connection with the ACL
	if server.ACL != nil {
		server.ACL.RegisterConnection(&conn)
	}

	w, r := io.Writer(conn), io.Reader(conn)

	cid := server.ConnID.Add(1)
	ctx = context.WithValue(ctx, utils.ContextConnID("ConnectionID"),
		fmt.Sprintf("%s-%d", ctx.Value(utils.ContextServerID("ServerID")), cid))

	for {
		message, err := utils.ReadMessage(r)

		if err != nil && errors.Is(err, io.EOF) {
			// Connection closed
			log.Println(err)
			break
		}

		if err != nil {
			log.Println(err)
			break
		}

		res, err := server.handleCommand(ctx, message, &conn, false)

		if err != nil && errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			if _, err = w.Write([]byte(fmt.Sprintf("-Error %s\r\n", err.Error()))); err != nil {
				log.Println(err)
			}
			continue
		}

		chunkSize := 1024

		// If the length of the response is 0, return nothing to the client
		if len(res) == 0 {
			continue
		}

		if len(res) <= chunkSize {
			_, _ = w.Write(res)
			continue
		}

		// If the response is large, send it in chunks.
		startIndex := 0
		for {
			// If the current start index is less than chunkSize from length, return the remaining bytes.
			if len(res)-1-startIndex < chunkSize {
				_, err = w.Write(res[startIndex:])
				if err != nil {
					log.Println(err)
				}
				break
			}
			n, _ := w.Write(res[startIndex : startIndex+chunkSize])
			if n < chunkSize {
				break
			}
			startIndex += chunkSize
		}
	}

	if err := conn.Close(); err != nil {
		log.Println(err)
	}
}

func (server *EchoVault) Start(ctx context.Context) {
	conf := server.Config

	if conf.TLS && len(conf.CertKeyPairs) <= 0 {
		log.Fatal("must provide certificate and key file paths for TLS mode")
		return
	}

	if server.isInCluster() {
		// Initialise raft and memberlist
		server.raft.RaftInit(ctx)
		server.memberList.MemberListInit(ctx)
		if server.raft.IsRaftLeader() {
			server.initialiseCaches()
		}
	}

	if !server.isInCluster() {
		server.initialiseCaches()
		// Restore from AOF by default if it's enabled
		if conf.RestoreAOF {
			err := server.AOFEngine.Restore()
			if err != nil {
				log.Println(err)
			}
		}

		// Restore from snapshot if snapshot restore is enabled and AOF restore is disabled
		if conf.RestoreSnapshot && !conf.RestoreAOF {
			err := server.SnapshotEngine.Restore()
			if err != nil {
				log.Println(err)
			}
		}
	}

	server.StartTCP(ctx)
}

func (server *EchoVault) TakeSnapshot() error {
	if server.SnapshotInProgress.Load() {
		return errors.New("snapshot already in progress")
	}

	go func() {
		if server.isInCluster() {
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

func (server *EchoVault) StartSnapshot() {
	server.SnapshotInProgress.Store(true)
}

func (server *EchoVault) FinishSnapshot() {
	server.SnapshotInProgress.Store(false)
}

func (server *EchoVault) SetLatestSnapshot(msec int64) {
	server.LatestSnapshotMilliseconds.Store(msec)
}

func (server *EchoVault) GetLatestSnapshot() int64 {
	return server.LatestSnapshotMilliseconds.Load()
}

func (server *EchoVault) StartRewriteAOF() {
	server.RewriteAOFInProgress.Store(true)
}

func (server *EchoVault) FinishRewriteAOF() {
	server.RewriteAOFInProgress.Store(false)
}

func (server *EchoVault) RewriteAOF() error {
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

func (server *EchoVault) ShutDown(ctx context.Context) {
	if server.isInCluster() {
		server.raft.RaftShutdown(ctx)
		server.memberList.MemberListShutdown(ctx)
	}
}

func (server *EchoVault) initialiseCaches() {
	// Set up LFU cache
	server.lfuCache = struct {
		mutex sync.Mutex
		cache eviction.CacheLFU
	}{
		mutex: sync.Mutex{},
		cache: eviction.NewCacheLFU(),
	}
	// set up LRU cache
	server.lruCache = struct {
		mutex sync.Mutex
		cache eviction.CacheLRU
	}{
		mutex: sync.Mutex{},
		cache: eviction.NewCacheLRU(),
	}
}
