package etc

import (
	"errors"
	"github.com/echovault/echovault/src/utils"
)

func setKeyFunc(cmd []string) ([]string, error) {
	if len(cmd) != 3 {
		return nil, errors.New(utils.WrongArgsResponse)
	}
	return []string{cmd[1]}, nil
}

func setNXKeyFunc(cmd []string) ([]string, error) {
	if len(cmd) != 3 {
		return nil, errors.New(utils.WrongArgsResponse)
	}
	return []string{cmd[1]}, nil
}

func msetKeyFunc(cmd []string) ([]string, error) {
	if len(cmd[1:])%2 != 0 {
		return nil, errors.New("each key must be paired with a value")
	}
	var keys []string
	for i, key := range cmd[1:] {
		if i%2 == 0 {
			keys = append(keys, key)
		}
	}
	return keys, nil
}
