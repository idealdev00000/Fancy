package memberlist

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/kelvinmwinuka/memstore/src/utils"
	"time"
)

type Delegate struct {
	options DelegateOpts
}

type DelegateOpts struct {
	config         utils.Config
	broadcastQueue *memberlist.TransmitLimitedQueue
	addVoter       func(id raft.ServerID, address raft.ServerAddress, prevIndex uint64, timeout time.Duration) error
	isRaftLeader   func() bool
	applyMutate    func(ctx context.Context, cmd []string) ([]byte, error)
}

func NewDelegate(opts DelegateOpts) *Delegate {
	return &Delegate{
		options: opts,
	}
}

// NodeMeta implements Delegate interface
func (delegate *Delegate) NodeMeta(limit int) []byte {
	meta := NodeMeta{
		ServerID: raft.ServerID(delegate.options.config.ServerID),
		RaftAddr: raft.ServerAddress(
			fmt.Sprintf("%s:%d", delegate.options.config.BindAddr, delegate.options.config.RaftBindPort)),
		MemberlistAddr: fmt.Sprintf("%s:%d", delegate.options.config.BindAddr, delegate.options.config.MemberListBindPort),
	}

	b, err := json.Marshal(&meta)

	if err != nil {
		return []byte("")
	}

	return b
}

// NotifyMsg implements Delegate interface
func (delegate *Delegate) NotifyMsg(msgBytes []byte) {
	var msg BroadcastMessage

	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		fmt.Print(err)
		return
	}

	switch msg.Action {
	case "RaftJoin":
		// If the current node is not the cluster leader, re-broadcast the message
		if !delegate.options.isRaftLeader() {
			delegate.options.broadcastQueue.QueueBroadcast(&msg)
			return
		}
		err := delegate.options.addVoter(msg.NodeMeta.ServerID, msg.NodeMeta.RaftAddr, 0, 0)
		if err != nil {
			fmt.Println(err)
		}
	case "MutateData":
		// If the current node is not a cluster leader, re-broadcast the message
		if !delegate.options.isRaftLeader() {
			delegate.options.broadcastQueue.QueueBroadcast(&msg)
			return
		}
		// Current node is the cluster leader, handle the mutation
		ctx := context.WithValue(
			context.WithValue(context.Background(), utils.ContextServerID("ServerID"), string(msg.ServerID)),
			utils.ContextConnID("ConnectionID"), msg.ConnId)

		cmd, err := utils.Decode(msg.Content)
		if err != nil {
			// TODO: Log error to configured logger
			fmt.Println(err)
			return
		}

		if _, err := delegate.options.applyMutate(ctx, cmd); err != nil {
			// TODO: Log error to configured logger
			fmt.Println(err)
		}
	}
}

// GetBroadcasts implements Delegate interface
func (delegate *Delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return delegate.options.broadcastQueue.GetBroadcasts(overhead, limit)
}

// LocalState implements Delegate interface
func (delegate *Delegate) LocalState(join bool) []byte {
	// No-Op
	return []byte("")
}

// MergeRemoteState implements Delegate interface
func (delegate *Delegate) MergeRemoteState(buf []byte, join bool) {
	// No-Op
}