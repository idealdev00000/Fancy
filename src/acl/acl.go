package acl

import (
	"encoding/json"
	"fmt"
	"github.com/kelvinmwinuka/memstore/src/utils"
	"gopkg.in/yaml.v3"
	"log"
	"net"
	"os"
	"path"
	"strings"
)

type Password struct {
	PasswordType  string `json:"PasswordType" yaml:"PasswordType"` // plaintext, SHA256
	PasswordValue string `json:"PasswordValue" yaml:"PasswordValue"`
}

type User struct {
	Username string `json:"Username" yaml:"Username"`
	Enabled  bool   `json:"Enabled" yaml:"Enabled"`

	Passwords []Password `json:"Passwords" yaml:"Passwords"`

	IncludedCategories []string `json:"IncludedCategories" yaml:"IncludedCategories"`
	ExcludedCategories []string `json:"ExcludedCategories" yaml:"ExcludedCategories"`

	IncludedCommands []string `json:"IncludedCommands" yaml:"IncludedCommands"`
	ExcludedCommands []string `json:"ExcludedCommands" yaml:"ExcludedCommands"`

	IncludedKeys      []string `json:"IncludedKeys" yaml:"IncludedKeys"`
	ExcludedKeys      []string `json:"ExcludedKeys" yaml:"ExcludedKeys"`
	IncludedReadKeys  []string `json:"IncludedReadKeys" yaml:"IncludedReadKeys"`
	IncludedWriteKeys []string `json:"IncludedWriteKeys" yaml:"IncludedWriteKeys"`

	IncludedPubSubChannels []string `json:"IncludedPubSubChannels" yaml:"IncludedPubSubChannels"`
	ExcludedPubSubChannels []string `json:"ExcludedPubSubChannels" yaml:"ExcludedPubSubChannels"`
}

type ACL struct {
	Users       []User
	Connections map[*net.Conn]*User
	Config      utils.Config
	Plugin      Plugin
}

func NewACL(config utils.Config) *ACL {
	var users []User

	// 1. Initialise default ACL user
	defaultUser := CreateUser("default", true)
	if config.RequirePass {
		defaultUser.Passwords = []Password{
			{
				PasswordType:  GetPasswordType(config.Password),
				PasswordValue: config.Password,
			},
		}
	}

	// 2. Read and parse the ACL config file and set the
	if config.AclConfig != "" {
		// Override acl configurations from file
		if f, err := os.Open(config.AclConfig); err != nil {
			panic(err)
		} else {
			defer func() {
				if err := f.Close(); err != nil {
					fmt.Println("acl config file close error: ", err)
				}
			}()

			ext := path.Ext(f.Name())

			if ext == ".json" {
				if err := json.NewDecoder(f).Decode(&users); err != nil {
					log.Fatal("could not load JSON ACL config: ", err)
				}
			}

			if ext == ".yaml" || ext == ".yml" {
				if err := yaml.NewDecoder(f).Decode(&users); err != nil {
					log.Fatal("could not load YAML ACL config: ", err)
				}
			}
		}
	}

	// 3. Merge created default user and loaded default user
	for i, user := range users {
		if user.Username == "default" {
			u, err := MergeUser(defaultUser, user)
			if err != nil {
				fmt.Println(err)
				continue
			}
			users[i] = u
		}
	}

	// 4. Normalise the list of users ACL Config.

	acl := ACL{
		Users:       users,
		Connections: make(map[*net.Conn]*User),
		Config:      config,
	}
	acl.Plugin = NewACLPlugin(&acl)

	fmt.Println(acl.Users)

	return &acl
}

func (acl *ACL) GetPluginCommands() []utils.Command {
	return acl.Plugin.GetCommands()
}

func (acl *ACL) RegisterConnection(conn *net.Conn) {
	fmt.Println("Register connection...")
}

func (acl *ACL) AuthenticateConnection(conn *net.Conn, cmd []string) error {
	return nil
}

func (acl *ACL) AuthorizeConnection(conn *net.Conn, cmd []string) error {
	return nil
}

func GetPasswordType(password string) string {
	if strings.Split(password, "")[0] == "#" {
		return "SHA256"
	}
	return "plaintext"
}

func CreateUser(username string, enabled bool) User {
	return User{
		Username:               username,
		Enabled:                enabled,
		Passwords:              []Password{},
		IncludedCategories:     []string{"*"},
		ExcludedCategories:     []string{},
		IncludedCommands:       []string{"*"},
		ExcludedCommands:       []string{},
		IncludedKeys:           []string{"*"},
		ExcludedKeys:           []string{},
		IncludedReadKeys:       []string{"*"},
		IncludedWriteKeys:      []string{"*"},
		IncludedPubSubChannels: []string{"*"},
		ExcludedPubSubChannels: []string{},
	}
}

func NormaliseUser(user User) User {
	// Normalise the user object
	return User{}
}

func MergeUser(base, target User) (User, error) {
	if base.Username != target.Username {
		return User{},
			fmt.Errorf("cannot merge user with username %s to user with username %s", base.Username, target.Username)
	}

	result := base

	result.Enabled = target.Enabled
	result.Passwords = append(base.Passwords, target.Passwords...)
	result.IncludedCategories = append(base.IncludedCategories, target.IncludedCategories...)
	result.ExcludedCategories = append(base.ExcludedCategories, target.ExcludedCategories...)
	result.IncludedCommands = append(base.IncludedCommands, target.IncludedCommands...)
	result.ExcludedCommands = append(base.ExcludedCommands, target.ExcludedCommands...)
	result.IncludedReadKeys = append(base.IncludedReadKeys, target.IncludedReadKeys...)
	result.IncludedWriteKeys = append(base.IncludedWriteKeys, target.IncludedWriteKeys...)
	result.IncludedPubSubChannels = append(base.IncludedPubSubChannels, target.IncludedPubSubChannels...)
	result.ExcludedPubSubChannels = append(base.ExcludedPubSubChannels, target.ExcludedPubSubChannels...)

	return NormaliseUser(result), nil
}
