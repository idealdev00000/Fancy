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

package internal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/echovault/echovault/pkg/utils"
	"io"
	"log"
	"math/big"
	"net"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sethvargo/go-retry"
	"github.com/tidwall/resp"
)

func AdaptType(s string) interface{} {
	// Adapt the type of the parameter to string, float64 or int
	n, _, err := big.ParseFloat(s, 10, 256, big.RoundingMode(big.Exact))

	if err != nil {
		return s
	}

	if n.IsInt() {
		i, _ := n.Int64()
		return int(i)
	}

	f, _ := n.Float64()

	return f
}

func Decode(raw []byte) ([]string, error) {
	reader := resp.NewReader(bytes.NewReader(raw))

	value, _, err := reader.ReadValue()
	if err != nil {
		return nil, err
	}

	var res []string
	for i := 0; i < len(value.Array()); i++ {
		res = append(res, value.Array()[i].String())
	}

	return res, nil
}

func ReadMessage(r io.Reader) ([]byte, error) {
	reader := bufio.NewReader(r)

	var res []byte

	chunk := make([]byte, 8192)

	for {
		n, err := reader.Read(chunk)
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		res = append(res, chunk...)
		if n < len(chunk) {
			break
		}
		clear(chunk)
	}

	return bytes.Trim(res, "\x00"), nil
}

func RetryBackoff(b retry.Backoff, maxRetries uint64, jitter, cappedDuration, maxDuration time.Duration) retry.Backoff {
	backoff := b
	if maxRetries > 0 {
		backoff = retry.WithMaxRetries(maxRetries, backoff)
	}
	if jitter > 0 {
		backoff = retry.WithJitter(jitter, backoff)
	}
	if cappedDuration > 0 {
		backoff = retry.WithCappedDuration(cappedDuration, backoff)
	}
	if maxDuration > 0 {
		backoff = retry.WithMaxDuration(maxDuration, backoff)
	}
	return backoff
}

func GetIPAddress() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer func() {
		if err = conn.Close(); err != nil {
			log.Println(err)
		}
	}()

	localAddr := strings.Split(conn.LocalAddr().String(), ":")[0]

	return localAddr, nil
}

func GetSubCommand(command utils.Command, cmd []string) interface{} {
	if len(command.SubCommands) == 0 || len(cmd) < 2 {
		return nil
	}
	for _, subCommand := range command.SubCommands {
		if strings.EqualFold(subCommand.Command, cmd[1]) {
			return subCommand
		}
	}
	return nil
}

func IsWriteCommand(command utils.Command, subCommand utils.SubCommand) bool {
	return slices.Contains(append(command.Categories, subCommand.Categories...), utils.WriteCategory)
}

func AbsInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ParseMemory returns an integer representing the bytes in the memory string
func ParseMemory(memory string) (uint64, error) {
	// Parse memory strings such as "100mb", "16gb"
	memString := memory[0 : len(memory)-2]
	bytesInt, err := strconv.ParseInt(memString, 10, 64)
	if err != nil {
		return 0, err
	}

	memUnit := strings.ToLower(memory[len(memory)-2:])
	switch memUnit {
	case "kb":
		bytesInt *= 1024
	case "mb":
		bytesInt *= 1024 * 1024
	case "gb":
		bytesInt *= 1024 * 1024 * 1024
	case "tb":
		bytesInt *= 1024 * 1024 * 1024 * 1024
	case "pb":
		bytesInt *= 1024 * 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("memory unit %s not supported, use (kb, mb, gb, tb, pb) ", memUnit)
	}

	return uint64(bytesInt), nil
}

// IsMaxMemoryExceeded checks whether we have exceeded the current maximum memory limit.
func IsMaxMemoryExceeded(maxMemory uint64) bool {
	if maxMemory == 0 {
		return false
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// If we're currently using less than the configured max memory, return false.
	if memStats.HeapInuse < maxMemory {
		return false
	}

	// If we're currently using more than max memory, force a garbage collection before we start deleting keys.
	// This measure is to prevent deleting keys that may be important when some memory can be reclaimed
	// by just collecting garbage.
	runtime.GC()
	runtime.ReadMemStats(&memStats)

	// Return true when whe are above or equal to max memory.
	return memStats.HeapInuse >= maxMemory
}

// FilterExpiredKeys filters out keys that are already expired, so they are not persisted.
func FilterExpiredKeys(state map[string]KeyData) map[string]KeyData {
	var keysToDelete []string
	for k, v := range state {
		// Skip keys with no expiry time.
		if v.ExpireAt == (time.Time{}) {
			continue
		}
		// If the key is already expired, mark it for deletion.
		if v.ExpireAt.Before(time.Now()) {
			keysToDelete = append(keysToDelete, k)
		}
	}
	for _, key := range keysToDelete {
		delete(state, key)
	}
	return state
}

func EncodeCommand(cmd []string) []byte {
	res := fmt.Sprintf("*%d\r\n", len(cmd))
	for _, token := range cmd {
		res += fmt.Sprintf("$%d\r\n%s\r\n", len(token), token)
	}
	return []byte(res)
}

func ParseNilResponse(b []byte) (bool, error) {
	r := resp.NewReader(bytes.NewReader(b))
	v, _, err := r.ReadValue()
	if err != nil {
		return false, err
	}
	return v.IsNull(), nil
}

func ParseStringResponse(b []byte) (string, error) {
	r := resp.NewReader(bytes.NewReader(b))
	v, _, err := r.ReadValue()
	if err != nil {
		return "", err
	}
	return v.String(), nil
}

func ParseIntegerResponse(b []byte) (int, error) {
	r := resp.NewReader(bytes.NewReader(b))
	v, _, err := r.ReadValue()
	if err != nil {
		return 0, err
	}
	return v.Integer(), nil
}

func ParseFloatResponse(b []byte) (float64, error) {
	r := resp.NewReader(bytes.NewReader(b))
	v, _, err := r.ReadValue()
	if err != nil {
		return 0, err
	}
	return v.Float(), nil
}

func ParseBooleanResponse(b []byte) (bool, error) {
	r := resp.NewReader(bytes.NewReader(b))
	v, _, err := r.ReadValue()
	if err != nil {
		return false, err
	}
	return v.Bool(), nil
}

func ParseStringArrayResponse(b []byte) ([]string, error) {
	r := resp.NewReader(bytes.NewReader(b))
	v, _, err := r.ReadValue()
	if err != nil {
		return nil, err
	}
	if v.IsNull() {
		return []string{}, nil
	}
	arr := make([]string, len(v.Array()))
	for i, e := range v.Array() {
		if e.IsNull() {
			arr[i] = ""
			continue
		}
		arr[i] = e.String()
	}
	return arr, nil
}

func ParseNestedStringArrayResponse(b []byte) ([][]string, error) {
	r := resp.NewReader(bytes.NewReader(b))
	v, _, err := r.ReadValue()
	if err != nil {
		return nil, err
	}
	if v.IsNull() {
		return [][]string{}, nil
	}
	arr := make([][]string, len(v.Array()))
	for i, e1 := range v.Array() {
		if e1.IsNull() {
			arr[i] = []string{}
			continue
		}
		entry := make([]string, len(e1.Array()))
		for j, e2 := range e1.Array() {
			entry[j] = e2.String()
		}
		arr[i] = entry
	}
	return arr, nil
}

func ParseIntegerArrayResponse(b []byte) ([]int, error) {
	r := resp.NewReader(bytes.NewReader(b))
	v, _, err := r.ReadValue()
	if err != nil {
		return nil, err
	}
	if v.IsNull() {
		return []int{}, nil
	}
	arr := make([]int, len(v.Array()))
	for i, e := range v.Array() {
		if e.IsNull() {
			arr[i] = 0
			continue
		}
		arr[i] = e.Integer()
	}
	return arr, nil
}

func ParseBooleanArrayResponse(b []byte) ([]bool, error) {
	r := resp.NewReader(bytes.NewReader(b))
	v, _, err := r.ReadValue()
	if err != nil {
		return nil, err
	}
	if v.IsNull() {
		return []bool{}, nil
	}
	arr := make([]bool, len(v.Array()))
	for i, e := range v.Array() {
		if e.IsNull() {
			arr[i] = false
			continue
		}
		arr[i] = e.Bool()
	}
	return arr, nil
}
