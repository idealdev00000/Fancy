package utils

import (
	"context"
	"net"
)

type ServerCommand struct {
	Command Command
	Plugin  Plugin
}

type Server interface {
	KeyLock(ctx context.Context, key string) (bool, error)
	KeyUnlock(key string)
	KeyRLock(ctx context.Context, key string) (bool, error)
	KeyRUnlock(key string)
	KeyExists(key string) bool
	CreateKeyAndLock(ctx context.Context, key string) (bool, error)
	GetValue(key string) interface{}
	SetValue(ctx context.Context, key string, value interface{})
	GetAllCommands(ctx context.Context) []Command
}

type ContextServerID string
type ContextConnID string

type ApplyRequest struct {
	ServerID     string   `json:"ServerID"`
	ConnectionID string   `json:"ConnectionID"`
	CMD          []string `json:"CMD"`
}

type ApplyResponse struct {
	Error    error
	Response []byte
}

type KeyExtractionFunc func(cmd []string) ([]string, error)

type SubCommand struct {
	Command     string
	Categories  []string
	Description string
	Sync        bool // Specifies if sub-command should be synced across cluster
	KeyExtractionFunc
}

type Command struct {
	Command     string
	Categories  []string
	Description string
	SubCommands []SubCommand
	Sync        bool // Specifies if command should be synced across cluster
	KeyExtractionFunc
}

type Plugin interface {
	Name() string
	Commands() []Command
	Description() string
	HandleCommand(ctx context.Context, cmd []string, server Server, conn *net.Conn) ([]byte, error)
}
