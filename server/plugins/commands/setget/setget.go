package main

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/kelvinmwinuka/memstore/utils"
)

type Server interface {
	Lock()
	Unlock()
	GetData(key string) interface{}
	SetData(key string, value interface{})
}

type plugin struct {
	name        string
	commands    []string
	description string
}

var Plugin plugin

func (p *plugin) Name() string {
	return p.name
}

func (p *plugin) Commands() []string {
	return p.commands
}

func (p *plugin) Description() string {
	return p.description
}

func (p *plugin) HandleCommand(cmd []string, server interface{}, conn *bufio.Writer) {
	switch strings.ToLower(cmd[0]) {
	case "get":
		handleGet(cmd, server.(Server), conn)
	case "set":
		handleSet(cmd, server.(Server), conn)
	case "mget":
		handleMGet(cmd, server.(Server), conn)
	}
}

func handleGet(cmd []string, s Server, conn *bufio.Writer) {

	if len(cmd) != 2 {
		conn.Write([]byte("-Error wrong number of args for GET command\r\n\n"))
		conn.Flush()
		return
	}

	s.Lock()
	value := s.GetData(cmd[1])
	s.Unlock()

	switch value.(type) {
	default:
		conn.Write([]byte(fmt.Sprintf("+%v\r\n\n", value)))
	case nil:
		conn.Write([]byte("+nil\r\n\n"))
	}
	conn.Flush()
}

func handleMGet(cmd []string, s Server, conn *bufio.Writer) {
	if len(cmd) < 2 {
		conn.Write([]byte("-Error wrong number of args for MGET command\r\n\n"))
		conn.Flush()
		return
	}

	vals := []string{}

	s.Lock()

	for _, key := range cmd[1:] {
		switch s.GetData(key).(type) {
		default:
			vals = append(vals, fmt.Sprintf("%v", s.GetData(key)))
		case nil:
			vals = append(vals, "nil")
		}
	}

	s.Unlock()

	conn.Write([]byte(fmt.Sprintf("*%d\r\n", len(vals))))

	for _, val := range vals {
		conn.Write([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(val), val)))
	}

	conn.Write([]byte("\n"))
	conn.Flush()
}

func handleSet(cmd []string, s Server, conn *bufio.Writer) {
	switch x := len(cmd); {
	default:
		conn.Write([]byte("-Error wrong number of args for SET command\r\n\n"))
		conn.Flush()
	case x == 3:
		s.Lock()
		s.SetData(cmd[1], utils.AdaptType(cmd[2]))
		s.Unlock()
		conn.Write([]byte("+OK\r\n\n"))
		conn.Flush()
	}
}

func init() {
	Plugin.name = "GetCommand"
	Plugin.commands = []string{"set", "get", "mget"}
	Plugin.description = "Handle basic SET, GET and MGET commands"
}
