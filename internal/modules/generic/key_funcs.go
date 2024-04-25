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

package generic

import (
	"errors"
	"github.com/echovault/echovault/internal"
	"github.com/echovault/echovault/pkg/constants"
)

func setKeyFunc(cmd []string) (internal.AccessKeys, error) {
	if len(cmd) < 3 || len(cmd) > 7 {
		return internal.AccessKeys{}, errors.New(constants.WrongArgsResponse)
	}
	return internal.AccessKeys{
		Channels:  make([]string, 0),
		ReadKeys:  make([]string, 0),
		WriteKeys: cmd[1:2],
	}, nil
}

func msetKeyFunc(cmd []string) (internal.AccessKeys, error) {
	if len(cmd[1:])%2 != 0 {
		return internal.AccessKeys{}, errors.New("each key must be paired with a value")
	}
	var keys []string
	for i, key := range cmd[1:] {
		if i%2 == 0 {
			keys = append(keys, key)
		}
	}
	return internal.AccessKeys{
		Channels:  make([]string, 0),
		ReadKeys:  make([]string, 0),
		WriteKeys: keys,
	}, nil
}

func getKeyFunc(cmd []string) (internal.AccessKeys, error) {
	if len(cmd) != 2 {
		return internal.AccessKeys{}, errors.New(constants.WrongArgsResponse)
	}
	return internal.AccessKeys{
		Channels:  make([]string, 0),
		ReadKeys:  cmd[1:],
		WriteKeys: make([]string, 0),
	}, nil
}

func mgetKeyFunc(cmd []string) (internal.AccessKeys, error) {
	if len(cmd) < 2 {
		return internal.AccessKeys{}, errors.New(constants.WrongArgsResponse)
	}
	return internal.AccessKeys{
		Channels:  make([]string, 0),
		ReadKeys:  cmd[1:],
		WriteKeys: make([]string, 0),
	}, nil
}

func delKeyFunc(cmd []string) (internal.AccessKeys, error) {
	if len(cmd) < 2 {
		return internal.AccessKeys{}, errors.New(constants.WrongArgsResponse)
	}
	return internal.AccessKeys{
		Channels:  make([]string, 0),
		ReadKeys:  make([]string, 0),
		WriteKeys: cmd[1:],
	}, nil
}

func persistKeyFunc(cmd []string) (internal.AccessKeys, error) {
	if len(cmd) != 2 {
		return internal.AccessKeys{}, errors.New(constants.WrongArgsResponse)
	}
	return internal.AccessKeys{
		Channels:  make([]string, 0),
		ReadKeys:  make([]string, 0),
		WriteKeys: cmd[1:],
	}, nil
}

func expireTimeKeyFunc(cmd []string) (internal.AccessKeys, error) {
	if len(cmd) != 2 {
		return internal.AccessKeys{}, errors.New(constants.WrongArgsResponse)
	}
	return internal.AccessKeys{
		Channels:  make([]string, 0),
		ReadKeys:  cmd[1:],
		WriteKeys: make([]string, 0),
	}, nil
}

func ttlKeyFunc(cmd []string) (internal.AccessKeys, error) {
	if len(cmd) != 2 {
		return internal.AccessKeys{}, errors.New(constants.WrongArgsResponse)
	}
	return internal.AccessKeys{
		Channels:  make([]string, 0),
		ReadKeys:  cmd[1:],
		WriteKeys: make([]string, 0),
	}, nil
}

func expireKeyFunc(cmd []string) (internal.AccessKeys, error) {
	if len(cmd) < 3 || len(cmd) > 4 {
		return internal.AccessKeys{}, errors.New(constants.WrongArgsResponse)
	}
	return internal.AccessKeys{
		Channels:  make([]string, 0),
		ReadKeys:  make([]string, 0),
		WriteKeys: cmd[1:2],
	}, nil
}

func expireAtKeyFunc(cmd []string) (internal.AccessKeys, error) {
	if len(cmd) < 3 || len(cmd) > 4 {
		return internal.AccessKeys{}, errors.New(constants.WrongArgsResponse)
	}
	return internal.AccessKeys{
		Channels:  make([]string, 0),
		ReadKeys:  make([]string, 0),
		WriteKeys: cmd[1:2],
	}, nil
}
