package main

import (
	"io"

	"github.com/hashicorp/raft"
)

func (server *Server) RaftInit() {
	// Triggered after MemberList init
}

func (server *Server) RaftShutdown() {
	// Triggered before MemberListShutdown
	// Leadership transfer if current node is the leader
	// Shutdown of the raft server
}

// Implement raft.FSM interface
func (server *Server) Apply(log *raft.Log) interface{} {
	return nil
}

// Implement raft.FSM interface
func (server *Server) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil
}

// Implement raft.FSM interface
func (server *Server) Restore(snapshot io.ReadCloser) error {
	return nil
}

// Implements raft.StableStore interface
func (server *Server) Set(key []byte, value []byte) error {
	return nil
}

// Implements raft.StableStore interface
func (server *Server) Get(key []byte) ([]byte, error) {
	return []byte{}, nil
}

// Implements raft.StableStore interface
func (server *Server) SetUint64(key []byte, val uint64) error {
	return nil
}

// Implements raft.StableStore interface
func (server *Server) GetUint64(key []byte) (uint64, error) {
	return 0, nil
}
