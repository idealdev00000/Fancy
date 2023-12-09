package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
)

type Server interface {
	KeyLock(ctx context.Context, key string) (bool, error)
	KeyUnlock(key string)
	KeyRLock(ctx context.Context, key string) (bool, error)
	KeyRUnlock(key string)
	KeyExists(key string) bool
	CreateKeyAndLock(ctx context.Context, key string) (bool, error)
	GetValue(key string) interface{}
	SetValue(ctx context.Context, key string, value interface{})
}

type Command struct {
	Command              string   `json:"Command"`
	Categories           []string `json:"Categories"`
	Description          string   `json:"Description"`
	HandleWithConnection bool     `json:"HandleWithConnection"`
	Sync                 bool     `json:"Sync"`
}

type plugin struct {
	name        string
	commands    []Command
	categories  []string
	description string
}

var Plugin plugin

func (p *plugin) Name() string {
	return p.name
}

func (p *plugin) Commands() ([]byte, error) {
	return json.Marshal(p.commands)
}

func (p *plugin) Description() string {
	return p.description
}

func (p *plugin) HandleCommandWithConnection(ctx context.Context, cmd []string, server interface{}, conn *net.Conn) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (p *plugin) HandleCommand(ctx context.Context, cmd []string, server interface{}) ([]byte, error) {
	switch strings.ToLower(cmd[0]) {
	default:
		return nil, errors.New("command unknown")
	case "get":
		return handleGet(ctx, cmd, server.(Server))
	case "mget":
		return handleMGet(ctx, cmd, server.(Server))
	}
}

func handleGet(ctx context.Context, cmd []string, s Server) ([]byte, error) {
	if len(cmd) != 2 {
		return nil, errors.New("wrong number of args for GET command")
	}

	key := cmd[1]

	s.KeyRLock(ctx, key)
	value := s.GetValue(key)
	s.KeyRUnlock(key)

	switch value.(type) {
	default:
		return []byte(fmt.Sprintf("+%v\r\n\n", value)), nil
	case nil:
		return []byte("+nil\r\n\n"), nil
	}
}

func handleMGet(ctx context.Context, cmd []string, s Server) ([]byte, error) {
	if len(cmd) < 2 {
		return nil, errors.New("wrong number of args for MGET command")
	}

	vals := []string{}

	for _, key := range cmd[1:] {
		func(key string) {
			s.KeyRLock(ctx, key)
			switch s.GetValue(key).(type) {
			default:
				vals = append(vals, fmt.Sprintf("%v", s.GetValue(key)))
			case nil:
				vals = append(vals, "nil")
			}
			s.KeyRUnlock(key)

		}(key)
	}

	var bytes []byte = []byte(fmt.Sprintf("*%d\r\n", len(vals)))

	for _, val := range vals {
		bytes = append(bytes, []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val))...)
	}

	bytes = append(bytes, []byte("\n")...)

	return bytes, nil
}

func init() {
	Plugin.name = "GetCommands"
	Plugin.commands = []Command{
		{
			Command:              "get",
			Categories:           []string{},
			Description:          "",
			HandleWithConnection: false,
			Sync:                 false,
		},
		{
			Command:              "mget",
			Categories:           []string{},
			Description:          "",
			HandleWithConnection: false,
			Sync:                 true,
		},
	}
	Plugin.description = "Handle basic GET and MGET commands"
}
