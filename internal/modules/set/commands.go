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

package set

import (
	"errors"
	"fmt"
	"github.com/echovault/echovault/internal"
	"github.com/echovault/echovault/pkg/constants"
	"github.com/echovault/echovault/pkg/types"
	"slices"
	"strings"
)

func handleSADD(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := saddKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	key := keys.WriteKeys[0]

	var set *Set

	if !params.KeyExists(params.Context, key) {
		set = NewSet(params.Command[2:])
		if ok, err := params.CreateKeyAndLock(params.Context, key); !ok && err != nil {
			return nil, err
		}
		if err = params.SetValue(params.Context, key, set); err != nil {
			return nil, err
		}
		params.KeyUnlock(params.Context, key)
		return []byte(fmt.Sprintf(":%d\r\n", len(params.Command[2:]))), nil
	}

	if _, err = params.KeyLock(params.Context, key); err != nil {
		return nil, err
	}
	defer params.KeyUnlock(params.Context, key)

	set, ok := params.GetValue(params.Context, key).(*Set)
	if !ok {
		return nil, fmt.Errorf("value at key %s is not a set", key)
	}

	count := set.Add(params.Command[2:])

	return []byte(fmt.Sprintf(":%d\r\n", count)), nil
}

func handleSCARD(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := scardKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	key := keys.ReadKeys[0]

	if !params.KeyExists(params.Context, key) {
		return []byte(fmt.Sprintf(":0\r\n")), nil
	}

	if _, err = params.KeyRLock(params.Context, key); err != nil {
		return nil, err
	}
	defer params.KeyRUnlock(params.Context, key)

	set, ok := params.GetValue(params.Context, key).(*Set)
	if !ok {
		return nil, fmt.Errorf("value at key %s is not a set", key)
	}

	cardinality := set.Cardinality()

	return []byte(fmt.Sprintf(":%d\r\n", cardinality)), nil
}

func handleSDIFF(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := sdiffKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	// Extract base set first
	if !params.KeyExists(params.Context, keys.ReadKeys[0]) {
		return nil, fmt.Errorf("key for base set \"%s\" does not exist", keys.ReadKeys[0])
	}
	if _, err = params.KeyRLock(params.Context, keys.ReadKeys[0]); err != nil {
		return nil, err
	}
	defer params.KeyRUnlock(params.Context, keys.ReadKeys[0])
	baseSet, ok := params.GetValue(params.Context, keys.ReadKeys[0]).(*Set)
	if !ok {
		return nil, fmt.Errorf("value at key %s is not a set", keys.ReadKeys[0])
	}

	locks := make(map[string]bool)
	defer func() {
		for key, locked := range locks {
			if locked {
				params.KeyRUnlock(params.Context, key)
			}
		}
	}()

	for _, key := range keys.ReadKeys[1:] {
		if !params.KeyExists(params.Context, key) {
			continue
		}
		if _, err = params.KeyRLock(params.Context, key); err != nil {
			continue
		}
		locks[key] = true
	}

	var sets []*Set
	for _, key := range params.Command[2:] {
		set, ok := params.GetValue(params.Context, key).(*Set)
		if !ok {
			continue
		}
		sets = append(sets, set)
	}

	diff := baseSet.Subtract(sets)
	elems := diff.GetAll()

	res := fmt.Sprintf("*%d", len(elems))
	for i, e := range elems {
		res = fmt.Sprintf("%s\r\n$%d\r\n%s", res, len(e), e)
		if i == len(elems)-1 {
			res += "\r\n"
		}
	}

	return []byte(res), nil
}

func handleSDIFFSTORE(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := sdiffstoreKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	destination := keys.WriteKeys[0]

	// Extract base set first
	if !params.KeyExists(params.Context, keys.ReadKeys[0]) {
		return nil, fmt.Errorf("key for base set \"%s\" does not exist", keys.ReadKeys[0])
	}
	if _, err := params.KeyRLock(params.Context, keys.ReadKeys[0]); err != nil {
		return nil, err
	}
	defer params.KeyRUnlock(params.Context, keys.ReadKeys[0])
	baseSet, ok := params.GetValue(params.Context, keys.ReadKeys[0]).(*Set)
	if !ok {
		return nil, fmt.Errorf("value at key %s is not a set", keys.ReadKeys[0])
	}

	locks := make(map[string]bool)
	defer func() {
		for key, locked := range locks {
			if locked {
				params.KeyRUnlock(params.Context, key)
			}
		}
	}()

	for _, key := range keys.ReadKeys[1:] {
		if !params.KeyExists(params.Context, key) {
			continue
		}
		if _, err = params.KeyRLock(params.Context, key); err != nil {
			continue
		}
		locks[key] = true
	}

	var sets []*Set
	for _, key := range keys.ReadKeys[1:] {
		set, ok := params.GetValue(params.Context, key).(*Set)
		if !ok {
			continue
		}
		sets = append(sets, set)
	}

	diff := baseSet.Subtract(sets)
	elems := diff.GetAll()

	res := fmt.Sprintf(":%d\r\n", len(elems))

	if params.KeyExists(params.Context, destination) {
		if _, err = params.KeyLock(params.Context, destination); err != nil {
			return nil, err
		}
		if err = params.SetValue(params.Context, destination, diff); err != nil {
			return nil, err
		}
		params.KeyUnlock(params.Context, destination)
		return []byte(res), nil
	}

	if _, err = params.CreateKeyAndLock(params.Context, destination); err != nil {
		return nil, err
	}
	if err = params.SetValue(params.Context, destination, diff); err != nil {
		return nil, err
	}
	params.KeyUnlock(params.Context, destination)

	return []byte(res), nil
}

func handleSINTER(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := sinterKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	locks := make(map[string]bool)
	defer func() {
		for key, locked := range locks {
			if locked {
				params.KeyRUnlock(params.Context, key)
			}
		}
	}()

	for _, key := range keys.ReadKeys {
		if !params.KeyExists(params.Context, key) {
			// If key does not exist, then there is no intersection
			return []byte("*0\r\n"), nil
		}
		if _, err = params.KeyRLock(params.Context, key); err != nil {
			return nil, err
		}
		locks[key] = true
	}

	var sets []*Set

	for key, _ := range locks {
		set, ok := params.GetValue(params.Context, key).(*Set)
		if !ok {
			// If the value at the key is not a set, return error
			return nil, fmt.Errorf("value at key %s is not a set", key)
		}
		sets = append(sets, set)
	}

	if len(sets) <= 0 {
		return nil, fmt.Errorf("not enough sets in the keys provided")
	}

	intersect, _ := Intersection(0, sets...)
	elems := intersect.GetAll()

	res := fmt.Sprintf("*%d", len(elems))
	for i, e := range elems {
		res = fmt.Sprintf("%s\r\n$%d\r\n%s", res, len(e), e)
		if i == len(elems)-1 {
			res += "\r\n"
		}
	}

	return []byte(res), nil
}

func handleSINTERCARD(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := sintercardKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	// Extract the limit from the command
	var limit int
	limitIdx := slices.IndexFunc(params.Command, func(s string) bool {
		return strings.EqualFold(s, "limit")
	})
	if limitIdx >= 0 && limitIdx < 2 {
		return nil, errors.New(constants.WrongArgsResponse)
	}
	if limitIdx != -1 {
		limitIdx += 1
		if limitIdx >= len(params.Command) {
			return nil, errors.New("provide limit after LIMIT keyword")
		}

		if l, ok := internal.AdaptType(params.Command[limitIdx]).(int); !ok {
			return nil, errors.New("limit must be an integer")
		} else {
			limit = l
		}
	}

	locks := make(map[string]bool)
	defer func() {
		for key, locked := range locks {
			if locked {
				params.KeyRUnlock(params.Context, key)
			}
		}
	}()

	for _, key := range keys.ReadKeys {
		if !params.KeyExists(params.Context, key) {
			// If key does not exist, then there is no intersection
			return []byte(":0\r\n"), nil
		}
		if _, err = params.KeyRLock(params.Context, key); err != nil {
			return nil, err
		}
		locks[key] = true
	}

	var sets []*Set

	for key, _ := range locks {
		set, ok := params.GetValue(params.Context, key).(*Set)
		if !ok {
			// If the value at the key is not a set, return error
			return nil, fmt.Errorf("value at key %s is not a set", key)
		}
		sets = append(sets, set)
	}

	if len(sets) <= 0 {
		return nil, fmt.Errorf("not enough sets in the keys provided")
	}

	intersect, _ := Intersection(limit, sets...)

	return []byte(fmt.Sprintf(":%d\r\n", intersect.Cardinality())), nil
}

func handleSINTERSTORE(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := sinterstoreKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	locks := make(map[string]bool)
	defer func() {
		for key, locked := range locks {
			if locked {
				params.KeyRUnlock(params.Context, key)
			}
		}
	}()

	for _, key := range keys.ReadKeys {
		if !params.KeyExists(params.Context, key) {
			// If key does not exist, then there is no intersection
			return []byte(":0\r\n"), nil
		}
		if _, err = params.KeyRLock(params.Context, key); err != nil {
			return nil, err
		}
		locks[key] = true
	}

	var sets []*Set

	for key, _ := range locks {
		set, ok := params.GetValue(params.Context, key).(*Set)
		if !ok {
			// If the value at the key is not a set, return error
			return nil, fmt.Errorf("value at key %s is not a set", key)
		}
		sets = append(sets, set)
	}

	intersect, _ := Intersection(0, sets...)
	destination := keys.WriteKeys[0]

	if params.KeyExists(params.Context, destination) {
		if _, err = params.KeyLock(params.Context, destination); err != nil {
			return nil, err
		}
	} else {
		if _, err = params.CreateKeyAndLock(params.Context, destination); err != nil {
			return nil, err
		}
	}

	if err = params.SetValue(params.Context, destination, intersect); err != nil {
		return nil, err
	}
	params.KeyUnlock(params.Context, destination)

	return []byte(fmt.Sprintf(":%d\r\n", intersect.Cardinality())), nil
}

func handleSISMEMBER(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := sismemberKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	key := keys.ReadKeys[0]

	if !params.KeyExists(params.Context, key) {
		return []byte(":0\r\n"), nil
	}

	if _, err = params.KeyRLock(params.Context, key); err != nil {
		return nil, err
	}
	defer params.KeyRUnlock(params.Context, key)

	set, ok := params.GetValue(params.Context, key).(*Set)
	if !ok {
		return nil, fmt.Errorf("value at key %s is not a set", key)
	}

	if !set.Contains(params.Command[2]) {
		return []byte(":0\r\n"), nil
	}

	return []byte(":1\r\n"), nil
}

func handleSMEMBERS(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := smembersKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	key := keys.ReadKeys[0]

	if !params.KeyExists(params.Context, key) {
		return []byte("*0\r\n"), nil
	}

	if _, err = params.KeyRLock(params.Context, key); err != nil {
		return nil, err
	}
	defer params.KeyRUnlock(params.Context, key)

	set, ok := params.GetValue(params.Context, key).(*Set)
	if !ok {
		return nil, fmt.Errorf("value at key %s is not a set", key)
	}

	elems := set.GetAll()

	res := fmt.Sprintf("*%d", len(elems))
	for i, e := range elems {
		res = fmt.Sprintf("%s\r\n$%d\r\n%s", res, len(e), e)
		if i == len(elems)-1 {
			res += "\r\n"
		}
	}

	return []byte(res), nil
}

func handleSMISMEMBER(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := smismemberKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	key := keys.ReadKeys[0]
	members := params.Command[2:]

	if !params.KeyExists(params.Context, key) {
		res := fmt.Sprintf("*%d", len(members))
		for i, _ := range members {
			res = fmt.Sprintf("%s\r\n:0", res)
			if i == len(members)-1 {
				res += "\r\n"
			}
		}
		return []byte(res), nil
	}

	if _, err = params.KeyRLock(params.Context, key); err != nil {
		return nil, err
	}
	defer params.KeyRUnlock(params.Context, key)

	set, ok := params.GetValue(params.Context, key).(*Set)
	if !ok {
		return nil, fmt.Errorf("value at key %s is not a set", key)
	}

	res := fmt.Sprintf("*%d", len(members))
	for i := 0; i < len(members); i++ {
		if set.Contains(members[i]) {
			res += "\r\n:1"
		} else {
			res += "\r\n:0"
		}
	}
	res += "\r\n"

	return []byte(res), nil
}

func handleSMOVE(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := smoveKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	source, destination := keys.WriteKeys[0], keys.WriteKeys[1]
	member := params.Command[3]

	if !params.KeyExists(params.Context, source) {
		return []byte(":0\r\n"), nil
	}

	if _, err = params.KeyLock(params.Context, source); err != nil {
		return nil, err
	}
	defer params.KeyUnlock(params.Context, source)

	sourceSet, ok := params.GetValue(params.Context, source).(*Set)
	if !ok {
		return nil, errors.New("source is not a set")
	}

	var destinationSet *Set

	if !params.KeyExists(params.Context, destination) {
		// Destination key does not exist
		if _, err = params.CreateKeyAndLock(params.Context, destination); err != nil {
			return nil, err
		}
		defer params.KeyUnlock(params.Context, destination)
		destinationSet = NewSet([]string{})
		if err = params.SetValue(params.Context, destination, destinationSet); err != nil {
			return nil, err
		}
	} else {
		// Destination key exists
		if _, err := params.KeyLock(params.Context, destination); err != nil {
			return nil, err
		}
		defer params.KeyUnlock(params.Context, destination)
		ds, ok := params.GetValue(params.Context, destination).(*Set)
		if !ok {
			return nil, errors.New("destination is not a set")
		}
		destinationSet = ds
	}

	res := sourceSet.Move(destinationSet, member)

	return []byte(fmt.Sprintf(":%d\r\n", res)), nil
}

func handleSPOP(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := spopKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	key := keys.WriteKeys[0]
	count := 1

	if len(params.Command) == 3 {
		c, ok := internal.AdaptType(params.Command[2]).(int)
		if !ok {
			return nil, errors.New("count must be an integer")
		}
		count = c
	}

	if !params.KeyExists(params.Context, key) {
		return []byte("*-1\r\n"), nil
	}

	if _, err = params.KeyLock(params.Context, key); err != nil {
		return nil, err
	}
	defer params.KeyUnlock(params.Context, key)

	set, ok := params.GetValue(params.Context, key).(*Set)
	if !ok {
		return nil, fmt.Errorf("value at %s is not a set", key)
	}

	members := set.Pop(count)

	res := fmt.Sprintf("*%d", len(members))
	for i, m := range members {
		res = fmt.Sprintf("%s\r\n$%d\r\n%s", res, len(m), m)
		if i == len(members)-1 {
			res += "\r\n"
		}
	}

	return []byte(res), nil
}

func handleSRANDMEMBER(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := srandmemberKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	key := keys.ReadKeys[0]
	count := 1

	if len(params.Command) == 3 {
		c, ok := internal.AdaptType(params.Command[2]).(int)
		if !ok {
			return nil, errors.New("count must be an integer")
		}
		count = c
	}

	if !params.KeyExists(params.Context, key) {
		return []byte("*-1\r\n"), nil
	}

	if _, err = params.KeyLock(params.Context, key); err != nil {
		return nil, err
	}
	defer params.KeyUnlock(params.Context, key)

	set, ok := params.GetValue(params.Context, key).(*Set)
	if !ok {
		return nil, fmt.Errorf("value at %s is not a set", key)
	}

	members := set.GetRandom(count)

	res := fmt.Sprintf("*%d", len(members))
	for i, m := range members {
		res = fmt.Sprintf("%s\r\n$%d\r\n%s", res, len(m), m)
		if i == len(members)-1 {
			res += "\r\n"
		}
	}

	return []byte(res), nil
}

func handleSREM(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := sremKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	key := keys.WriteKeys[0]
	members := params.Command[2:]

	if !params.KeyExists(params.Context, key) {
		return []byte(":0\r\n"), nil
	}

	if _, err = params.KeyLock(params.Context, key); err != nil {
		return nil, err
	}
	defer params.KeyUnlock(params.Context, key)

	set, ok := params.GetValue(params.Context, key).(*Set)
	if !ok {
		return nil, fmt.Errorf("value at key %s is not a set", key)
	}

	count := set.Remove(members)

	return []byte(fmt.Sprintf(":%d\r\n", count)), nil
}

func handleSUNION(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := sunionKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	locks := make(map[string]bool)
	defer func() {
		for key, locked := range locks {
			if locked {
				params.KeyRUnlock(params.Context, key)
			}
		}
	}()

	for _, key := range keys.ReadKeys {
		if !params.KeyExists(params.Context, key) {
			continue
		}
		if _, err = params.KeyRLock(params.Context, key); err != nil {
			return nil, err
		}
		locks[key] = true
	}

	var sets []*Set

	for key, locked := range locks {
		if !locked {
			continue
		}
		set, ok := params.GetValue(params.Context, key).(*Set)
		if !ok {
			return nil, fmt.Errorf("value at key %s is not a set", key)
		}
		sets = append(sets, set)
	}

	union := Union(sets...)

	res := fmt.Sprintf("*%d", union.Cardinality())
	for i, e := range union.GetAll() {
		res = fmt.Sprintf("%s\r\n$%d\r\n%s", res, len(e), e)
		if i == len(union.GetAll())-1 {
			res += "\r\n"
		}
	}

	return []byte(res), nil
}

func handleSUNIONSTORE(params types.HandlerFuncParams) ([]byte, error) {
	keys, err := sunionstoreKeyFunc(params.Command)
	if err != nil {
		return nil, err
	}

	locks := make(map[string]bool)
	defer func() {
		for key, locked := range locks {
			if locked {
				params.KeyRUnlock(params.Context, key)
			}
		}
	}()

	for _, key := range keys.ReadKeys {
		if !params.KeyExists(params.Context, key) {
			continue
		}
		if _, err = params.KeyRLock(params.Context, key); err != nil {
			return nil, err
		}
		locks[key] = true
	}

	var sets []*Set

	for key, locked := range locks {
		if !locked {
			continue
		}
		set, ok := params.GetValue(params.Context, key).(*Set)
		if !ok {
			return nil, fmt.Errorf("value at key %s is not a set", key)
		}
		sets = append(sets, set)
	}

	union := Union(sets...)

	destination := keys.WriteKeys[0]

	if params.KeyExists(params.Context, destination) {
		if _, err = params.KeyLock(params.Context, destination); err != nil {
			return nil, err
		}
	} else {
		if _, err = params.CreateKeyAndLock(params.Context, destination); err != nil {
			return nil, err
		}
	}
	defer params.KeyUnlock(params.Context, destination)

	if err = params.SetValue(params.Context, destination, union); err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(":%d\r\n", union.Cardinality())), nil
}

func Commands() []types.Command {
	return []types.Command{
		{
			Command:           "sadd",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.WriteCategory, constants.FastCategory},
			Description:       "(SADD key member [member...]) Add one or more members to the set. If the set does not exist, it's created.",
			Sync:              true,
			KeyExtractionFunc: saddKeyFunc,
			HandlerFunc:       handleSADD,
		},
		{
			Command:           "scard",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.WriteCategory, constants.FastCategory},
			Description:       "(SCARD key) Returns the cardinality of the set.",
			Sync:              false,
			KeyExtractionFunc: scardKeyFunc,
			HandlerFunc:       handleSCARD,
		},
		{
			Command:    "sdiff",
			Module:     constants.SetModule,
			Categories: []string{constants.SetCategory, constants.ReadCategory, constants.SlowCategory},
			Description: `(SDIFF key [key...]) Returns the difference between all the sets in the given keys.
If the first key provided is the only valid set, then this key's set will be returned as the result.
All keys that are non-existed or hold values that are not sets will be skipped.`,
			Sync:              false,
			KeyExtractionFunc: sdiffKeyFunc,
			HandlerFunc:       handleSDIFF,
		},
		{
			Command:    "sdiffstore",
			Module:     constants.SetModule,
			Categories: []string{constants.SetCategory, constants.WriteCategory, constants.SlowCategory},
			Description: `(SDIFFSTORE destination key [key...]) Works the same as SDIFF but also stores the result at 'destination'.
Returns the cardinality of the new set`,
			Sync:              true,
			KeyExtractionFunc: sdiffstoreKeyFunc,
			HandlerFunc:       handleSDIFFSTORE,
		},
		{
			Command:           "sinter",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.WriteCategory, constants.SlowCategory},
			Description:       "(SINTER key [key...]) Returns the intersection of multiple sets.",
			Sync:              false,
			KeyExtractionFunc: sinterKeyFunc,
			HandlerFunc:       handleSINTER,
		},
		{
			Command:           "sintercard",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.ReadCategory, constants.SlowCategory},
			Description:       "(SINTERCARD key [key...] [LIMIT limit]) Returns the cardinality of the intersection between multiple sets.",
			Sync:              false,
			KeyExtractionFunc: sintercardKeyFunc,
			HandlerFunc:       handleSINTERCARD,
		},
		{
			Command:           "sinterstore",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.WriteCategory, constants.SlowCategory},
			Description:       "(SINTERSTORE destination key [key...]) Stores the intersection of multiple sets at the destination key.",
			Sync:              true,
			KeyExtractionFunc: sinterstoreKeyFunc,
			HandlerFunc:       handleSINTERSTORE,
		},
		{
			Command:           "sismember",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.ReadCategory, constants.FastCategory},
			Description:       "(SISMEMBER key member) Returns if member is contained in the set.",
			Sync:              false,
			KeyExtractionFunc: sismemberKeyFunc,
			HandlerFunc:       handleSISMEMBER,
		},
		{
			Command:           "smembers",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.ReadCategory, constants.SlowCategory},
			Description:       "(SMEMBERS key) Returns all members of a set.",
			Sync:              false,
			KeyExtractionFunc: smembersKeyFunc,
			HandlerFunc:       handleSMEMBERS,
		},
		{
			Command:           "smismember",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.ReadCategory, constants.FastCategory},
			Description:       "(SMISMEMBER key member [member...]) Returns if multiple members are in the set.",
			Sync:              false,
			KeyExtractionFunc: smismemberKeyFunc,
			HandlerFunc:       handleSMISMEMBER,
		},

		{
			Command:           "smove",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.WriteCategory, constants.FastCategory},
			Description:       "(SMOVE source destination member) Moves a member from source set to destination set.",
			Sync:              true,
			KeyExtractionFunc: smoveKeyFunc,
			HandlerFunc:       handleSMOVE,
		},
		{
			Command:           "spop",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.WriteCategory, constants.SlowCategory},
			Description:       "(SPOP key [count]) Returns and removes one or more random members from the set.",
			Sync:              true,
			KeyExtractionFunc: spopKeyFunc,
			HandlerFunc:       handleSPOP,
		},
		{
			Command:           "srandmember",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.ReadCategory, constants.SlowCategory},
			Description:       "(SRANDMEMBER key [count]) Returns one or more random members from the set without removing them.",
			Sync:              false,
			KeyExtractionFunc: srandmemberKeyFunc,
			HandlerFunc:       handleSRANDMEMBER,
		},
		{
			Command:           "srem",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.WriteCategory, constants.FastCategory},
			Description:       "(SREM key member [member...]) Remove one or more members from a set.",
			Sync:              true,
			KeyExtractionFunc: sremKeyFunc,
			HandlerFunc:       handleSREM,
		},
		{
			Command:           "sunion",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.ReadCategory, constants.SlowCategory},
			Description:       "(SUNION key [key...]) Returns the members of the set resulting from the union of the provided sets.",
			Sync:              false,
			KeyExtractionFunc: sunionKeyFunc,
			HandlerFunc:       handleSUNION,
		},
		{
			Command:           "sunionstore",
			Module:            constants.SetModule,
			Categories:        []string{constants.SetCategory, constants.WriteCategory, constants.SlowCategory},
			Description:       "(SUNIONSTORE destination key [key...]) Stores the union of the given sets into destination.",
			Sync:              true,
			KeyExtractionFunc: sunionstoreKeyFunc,
			HandlerFunc:       handleSUNIONSTORE,
		},
	}
}
