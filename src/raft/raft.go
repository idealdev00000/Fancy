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

package raft

import (
	"context"
	"errors"
	"fmt"
	"github.com/echovault/echovault/src/memberlist"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/echovault/echovault/src/utils"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

type Opts struct {
	Config     utils.Config
	EchoVault  utils.EchoVault
	GetCommand func(command string) (utils.Command, error)
	DeleteKey  func(ctx context.Context, key string) error
}

type Raft struct {
	options Opts
	raft    *raft.Raft
}

func NewRaft(opts Opts) *Raft {
	return &Raft{
		options: opts,
	}
}

func (r *Raft) RaftInit(ctx context.Context) {
	conf := r.options.Config

	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(conf.ServerID)
	raftConfig.SnapshotThreshold = conf.SnapShotThreshold
	raftConfig.SnapshotInterval = conf.SnapshotInterval

	var logStore raft.LogStore
	var stableStore raft.StableStore
	var snapshotStore raft.SnapshotStore

	if conf.InMemory {
		logStore = raft.NewInmemStore()
		stableStore = raft.NewInmemStore()
		snapshotStore = raft.NewInmemSnapshotStore()
	} else {
		boltdb, err := raftboltdb.NewBoltStore(filepath.Join(conf.DataDir, "logs.db"))
		if err != nil {
			log.Fatal(err)
		}

		logStore, err = raft.NewLogCache(512, boltdb)
		if err != nil {
			log.Fatal(err)
		}

		stableStore = raft.StableStore(boltdb)

		snapshotStore, err = raft.NewFileSnapshotStore(conf.DataDir, 2, os.Stdout)
		if err != nil {
			log.Fatal(err)
		}
	}

	addr := fmt.Sprintf("%s:%d", conf.BindAddr, conf.RaftBindPort)

	advertiseAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	raftTransport, err := raft.NewTCPTransport(
		addr,
		advertiseAddr,
		10,
		500*time.Millisecond,
		os.Stdout,
	)

	if err != nil {
		log.Fatal(err)
	}

	// Start raft echovault
	raftServer, err := raft.NewRaft(
		raftConfig,
		NewFSM(FSMOpts{
			Config:     r.options.Config,
			EchoVault:  r.options.EchoVault,
			GetCommand: r.options.GetCommand,
			DeleteKey:  r.options.DeleteKey,
		}),
		logStore,
		stableStore,
		snapshotStore,
		raftTransport,
	)

	if err != nil {
		log.Fatalf("Could not start node with error; %s", err)
	}

	if conf.BootstrapCluster {
		// Error can be safely ignored if we're already leader
		_ = raftServer.BootstrapCluster(raft.Configuration{
			Servers: []raft.Server{
				{
					Suffrage: raft.Voter,
					ID:       raft.ServerID(conf.ServerID),
					Address:  raft.ServerAddress(addr),
				},
			},
		}).Error()
	}

	r.raft = raftServer
}

func (r *Raft) Apply(cmd []byte, timeout time.Duration) raft.ApplyFuture {
	return r.raft.Apply(cmd, timeout)
}

func (r *Raft) IsRaftLeader() bool {
	return r.raft.State() == raft.Leader
}

func (r *Raft) isRaftFollower() bool {
	return r.raft.State() == raft.Follower
}

func (r *Raft) HasJoinedCluster() bool {
	isFollower := r.isRaftFollower()

	leaderAddr, leaderID := r.raft.LeaderWithID()
	hasLeader := leaderAddr != "" && leaderID != ""

	return isFollower && hasLeader
}

func (r *Raft) AddVoter(
	id raft.ServerID,
	address raft.ServerAddress,
	prevIndex uint64,
	timeout time.Duration,
) error {
	if r.IsRaftLeader() {
		raftConfig := r.raft.GetConfiguration()
		if err := raftConfig.Error(); err != nil {
			return errors.New("could not retrieve raft config")
		}

		for _, s := range raftConfig.Configuration().Servers {
			// Check if a echovault already exists with the current attributes
			if s.ID == id && s.Address == address {
				return fmt.Errorf("echovault with id %s and address %s already exists", id, address)
			}
		}

		err := r.raft.AddVoter(id, address, prevIndex, timeout).Error()
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Raft) RemoveServer(meta memberlist.NodeMeta) error {
	if !r.IsRaftLeader() {
		return errors.New("not leader, could not remove echovault")
	}

	if err := r.raft.RemoveServer(meta.ServerID, 0, 0).Error(); err != nil {
		return err
	}

	return nil
}

func (r *Raft) TakeSnapshot() error {
	return r.raft.Snapshot().Error()
}

func (r *Raft) RaftShutdown(ctx context.Context) {
	// Leadership transfer if current node is the leader
	if r.IsRaftLeader() {
		err := r.raft.LeadershipTransfer().Error()

		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Leadership transfer successful.")
	}
}
