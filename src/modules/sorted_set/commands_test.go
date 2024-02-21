package sorted_set

import (
	"bytes"
	"context"
	"errors"
	"github.com/echovault/echovault/src/server"
	"github.com/echovault/echovault/src/utils"
	"github.com/tidwall/resp"
	"math"
	"slices"
	"strconv"
	"testing"
)

func Test_HandleZADD(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValue      *SortedSet
		key              string
		command          []string
		expectedValue    *SortedSet
		expectedResponse int
		expectedError    error
	}{
		{ // 1. Create new sorted set and return the cardinality of the new sorted set.
			preset:      false,
			presetValue: nil,
			key:         "key1",
			command:     []string{"ZADD", "key1", "5.5", "member1", "67.77", "member2", "10", "member3", "-inf", "member4", "+inf", "member5"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
				{value: "member4", score: Score(math.Inf(-1))},
				{value: "member5", score: Score(math.Inf(1))},
			}),
			expectedResponse: 5,
			expectedError:    nil,
		},
		{ // 2. Only add the elements that do not currently exist in the sorted set when NX flag is provided
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			key:     "key2",
			command: []string{"ZADD", "key2", "NX", "5.5", "member1", "67.77", "member4", "10", "member5"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
				{value: "member4", score: Score(67.77)},
				{value: "member5", score: Score(10)},
			}),
			expectedResponse: 2,
			expectedError:    nil,
		},
		{ // 3. Do not add any elements when providing existing members with NX flag
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			key:     "key3",
			command: []string{"ZADD", "key3", "NX", "5.5", "member1", "67.77", "member2", "10", "member3"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // 4. Successfully add elements to an existing set when XX flag is provided with existing elements
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			key:     "key4",
			command: []string{"ZADD", "key4", "XX", "CH", "55", "member1", "1005", "member2", "15", "member3", "99.75", "member4"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(55)},
				{value: "member2", score: Score(1005)},
				{value: "member3", score: Score(15)},
			}),
			expectedResponse: 3,
			expectedError:    nil,
		},
		{ // 5. Fail to add element when providing XX flag with elements that do not exist in the sorted set.
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			key:     "key5",
			command: []string{"ZADD", "key5", "XX", "5.5", "member4", "100.5", "member5", "15", "member6"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // 6. Only update the elements where provided score is greater than current score if GT flag
			// Return only the new elements added by default
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			key:     "key6",
			command: []string{"ZADD", "key6", "XX", "CH", "GT", "7.5", "member1", "100.5", "member4", "15", "member5"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(7.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			expectedResponse: 1,
			expectedError:    nil,
		},
		{ // 7. Only update the elements where provided score is less than current score if LT flag is provided
			// Return only the new elements added by default.
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			key:     "key7",
			command: []string{"ZADD", "key7", "XX", "LT", "3.5", "member1", "100.5", "member4", "15", "member5"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(3.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // 8. Return all the elements that were updated AND added when CH flag is provided
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			key:     "key8",
			command: []string{"ZADD", "key8", "XX", "LT", "CH", "3.5", "member1", "100.5", "member4", "15", "member5"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(3.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			expectedResponse: 1,
			expectedError:    nil,
		},
		{ // 9. Increment the member by score
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			key:     "key9",
			command: []string{"ZADD", "key9", "INCR", "5.5", "member3"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(15.5)},
			}),
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // 10. Fail when GT/LT flag is provided alongside NX flag
			preset:           false,
			presetValue:      nil,
			key:              "key10",
			command:          []string{"ZADD", "key10", "NX", "LT", "CH", "3.5", "member1", "100.5", "member4", "15", "member5"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("GT/LT flags not allowed if NX flag is provided"),
		},
		{ // 11. Command is too short
			preset:           false,
			presetValue:      nil,
			key:              "key11",
			command:          []string{"ZADD", "key11"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
		{ // 12. Throw error when score/member entries are do not match
			preset:           false,
			presetValue:      nil,
			key:              "key11",
			command:          []string{"ZADD", "key12", "10.5", "member1", "12.5"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("score/member pairs must be float/string"),
		},
		{ // 13. Throw error when INCR flag is passed with more than one score/member pair
			preset:           false,
			presetValue:      nil,
			key:              "key13",
			command:          []string{"ZADD", "key13", "INCR", "10.5", "member1", "12.5", "member2"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("cannot pass more than one score/member pair when INCR flag is provided"),
		},
	}

	for _, test := range tests {
		if test.preset {
			if _, err := mockServer.CreateKeyAndLock(context.Background(), test.key); err != nil {
				t.Error(err)
			}
			mockServer.SetValue(context.Background(), test.key, test.presetValue)
			mockServer.KeyUnlock(test.key)
		}
		res, err := handleZADD(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		if err != nil {
			t.Error(err)
		}
		rd := resp.NewReader(bytes.NewReader(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		if rv.Integer() != test.expectedResponse {
			t.Errorf("expected response %d at key \"%s\", got %d", test.expectedResponse, test.key, rv.Integer())
		}
		// Fetch the sorted set from the server and check it against the expected result
		if _, err = mockServer.KeyRLock(context.Background(), test.key); err != nil {
			t.Error(err)
		}
		sortedSet, ok := mockServer.GetValue(test.key).(*SortedSet)
		if !ok {
			t.Errorf("expected the value at key \"%s\" to be a sorted set, got another type", test.key)
		}
		if test.expectedValue == nil {
			continue
		}
		for _, member := range sortedSet.GetAll() {
			expectedMember := test.expectedValue.Get(member.value)
			if !expectedMember.exists {
				t.Errorf("could not find member %+v in expected sorted set, found in stored set", member)
			}
			if member.score != expectedMember.score {
				t.Errorf("expected member \"%s\" to have score %f, got score %f", expectedMember.value, expectedMember.score, member.score)
			}
		}
		mockServer.KeyRUnlock(test.key)
	}
}

func Test_HandleZCARD(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValue      interface{}
		key              string
		command          []string
		expectedValue    *SortedSet
		expectedResponse int
		expectedError    error
	}{
		{ // 1. Get cardinality of valid sorted set.
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
			}),
			key:              "key1",
			command:          []string{"ZCARD", "key1"},
			expectedValue:    nil,
			expectedResponse: 3,
			expectedError:    nil,
		},
		{ // 2. Return 0 when trying to get cardinality from non-existent key
			preset:           false,
			presetValue:      nil,
			key:              "key2",
			command:          []string{"ZCARD", "key2"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // 3. Command is too short
			preset:           false,
			presetValue:      nil,
			key:              "key3",
			command:          []string{"ZCARD"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
		{ // 4. Command too long
			preset:           false,
			presetValue:      nil,
			key:              "key4",
			command:          []string{"ZCARD", "key4", "key5"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
		{ // 5. Return error when not a sorted set
			preset:           true,
			presetValue:      "Default value",
			key:              "key5",
			command:          []string{"ZCARD", "key5"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("value at key5 is not a sorted set"),
		},
	}

	for _, test := range tests {
		if test.preset {
			if _, err := mockServer.CreateKeyAndLock(context.Background(), test.key); err != nil {
				t.Error(err)
			}
			mockServer.SetValue(context.Background(), test.key, test.presetValue)
			mockServer.KeyUnlock(test.key)
		}
		res, err := handleZCARD(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		if err != nil {
			t.Error(err)
		}
		rd := resp.NewReader(bytes.NewReader(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		if rv.Integer() != test.expectedResponse {
			t.Errorf("expected response %d at key \"%s\", got %d", test.expectedResponse, test.key, rv.Integer())
		}
	}
}

func Test_HandleZCOUNT(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValue      interface{}
		key              string
		command          []string
		expectedValue    *SortedSet
		expectedResponse int
		expectedError    error
	}{
		{ // 1. Get entire count using infinity boundaries
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
				{value: "member4", score: Score(1083.13)},
				{value: "member5", score: Score(11)},
				{value: "member6", score: Score(math.Inf(-1))},
				{value: "member7", score: Score(math.Inf(1))},
			}),
			key:              "key1",
			command:          []string{"ZCOUNT", "key1", "-inf", "+inf"},
			expectedValue:    nil,
			expectedResponse: 7,
			expectedError:    nil,
		},
		{ // 2. Get count of sub-set from -inf to limit
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
				{value: "member4", score: Score(1083.13)},
				{value: "member5", score: Score(11)},
				{value: "member6", score: Score(math.Inf(-1))},
				{value: "member7", score: Score(math.Inf(1))},
			}),
			key:              "key2",
			command:          []string{"ZCOUNT", "key2", "-inf", "90"},
			expectedValue:    nil,
			expectedResponse: 5,
			expectedError:    nil,
		},
		{ // 3. Get count of sub-set from bottom boundary to +inf limit
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "member1", score: Score(5.5)},
				{value: "member2", score: Score(67.77)},
				{value: "member3", score: Score(10)},
				{value: "member4", score: Score(1083.13)},
				{value: "member5", score: Score(11)},
				{value: "member6", score: Score(math.Inf(-1))},
				{value: "member7", score: Score(math.Inf(1))},
			}),
			key:              "key3",
			command:          []string{"ZCOUNT", "key3", "1000", "+inf"},
			expectedValue:    nil,
			expectedResponse: 2,
			expectedError:    nil,
		},
		{ // 4. Return error when bottom boundary is not a valid double/float
			preset:           false,
			presetValue:      nil,
			key:              "key4",
			command:          []string{"ZCOUNT", "key4", "min", "10"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("min constraint must be a double"),
		},
		{ // 5. Return error when top boundary is not a valid double/float
			preset:           false,
			presetValue:      nil,
			key:              "key5",
			command:          []string{"ZCOUNT", "key5", "-10", "max"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("max constraint must be a double"),
		},
		{ // 6. Command is too short
			preset:           false,
			presetValue:      nil,
			key:              "key6",
			command:          []string{"ZCOUNT"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
		{ // 7. Command too long
			preset:           false,
			presetValue:      nil,
			key:              "key7",
			command:          []string{"ZCOUNT", "key4", "min", "max", "count"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
		{ // 8. Throw error when value at the key is not a sorted set
			preset:           true,
			presetValue:      "Default value",
			key:              "key8",
			command:          []string{"ZCOUNT", "key8", "1", "10"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("value at key8 is not a sorted set"),
		},
	}

	for _, test := range tests {
		if test.preset {
			if _, err := mockServer.CreateKeyAndLock(context.Background(), test.key); err != nil {
				t.Error(err)
			}
			mockServer.SetValue(context.Background(), test.key, test.presetValue)
			mockServer.KeyUnlock(test.key)
		}
		res, err := handleZCOUNT(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		if err != nil {
			t.Error(err)
		}
		rd := resp.NewReader(bytes.NewReader(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		if rv.Integer() != test.expectedResponse {
			t.Errorf("expected response %d at key \"%s\", got %d", test.expectedResponse, test.key, rv.Integer())
		}
	}
}

func Test_HandleZLEXCOUNT(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValue      interface{}
		key              string
		command          []string
		expectedValue    *SortedSet
		expectedResponse int
		expectedError    error
	}{
		{ // 1. Get entire count using infinity boundaries
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "e", score: Score(1)},
				{value: "f", score: Score(1)},
				{value: "g", score: Score(1)},
				{value: "h", score: Score(1)},
				{value: "i", score: Score(1)},
				{value: "j", score: Score(1)},
				{value: "k", score: Score(1)},
			}),
			key:              "key1",
			command:          []string{"ZLEXCOUNT", "key1", "f", "j"},
			expectedValue:    nil,
			expectedResponse: 5,
			expectedError:    nil,
		},
		{ // 2. Return 0 when the members do not have the same score
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "a", score: Score(5.5)},
				{value: "b", score: Score(67.77)},
				{value: "c", score: Score(10)},
				{value: "d", score: Score(1083.13)},
				{value: "e", score: Score(11)},
				{value: "f", score: Score(math.Inf(-1))},
				{value: "g", score: Score(math.Inf(1))},
			}),
			key:              "key2",
			command:          []string{"ZLEXCOUNT", "key2", "a", "b"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // 3. Return 0 when the key does not exist
			preset:           false,
			presetValue:      nil,
			key:              "key3",
			command:          []string{"ZLEXCOUNT", "key3", "a", "z"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // 4. Return error when the value at the key is not a sorted set
			preset:           true,
			presetValue:      "Default value",
			key:              "key4",
			command:          []string{"ZLEXCOUNT", "key4", "a", "z"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("value at key4 is not a sorted set"),
		},
		{ // 5. Command is too short
			preset:           false,
			presetValue:      nil,
			key:              "key5",
			command:          []string{"ZLEXCOUNT"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
		{ // 6. Command too long
			preset:           false,
			presetValue:      nil,
			key:              "key6",
			command:          []string{"ZLEXCOUNT", "key6", "min", "max", "count"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			if _, err := mockServer.CreateKeyAndLock(context.Background(), test.key); err != nil {
				t.Error(err)
			}
			mockServer.SetValue(context.Background(), test.key, test.presetValue)
			mockServer.KeyUnlock(test.key)
		}
		res, err := handleZLEXCOUNT(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		if err != nil {
			t.Error(err)
		}
		rd := resp.NewReader(bytes.NewReader(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		if rv.Integer() != test.expectedResponse {
			t.Errorf("expected response %d at key \"%s\", got %d", test.expectedResponse, test.key, rv.Integer())
		}
	}
}

func Test_HandleZDIFF(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValues     map[string]interface{}
		command          []string
		expectedResponse []string
		expectedError    error
	}{
		{ // 1. Get the difference between 2 sorted sets without scores.
			preset: true,
			presetValues: map[string]interface{}{
				"key1": NewSortedSet([]MemberParam{
					{value: "one", score: 1},
					{value: "two", score: 2},
					{value: "three", score: 3},
					{value: "four", score: 4},
				}),
				"key2": NewSortedSet([]MemberParam{
					{value: "three", score: 3},
					{value: "four", score: 4},
					{value: "five", score: 5},
					{value: "six", score: 6},
					{value: "seven", score: 7},
					{value: "eight", score: 8},
				}),
			},
			command:          []string{"ZDIFF", "key1", "key2"},
			expectedResponse: []string{"one", "two"},
			expectedError:    nil,
		},
		{ // 2. Get the difference between 2 sorted sets with scores.
			preset: true,
			presetValues: map[string]interface{}{
				"key1": NewSortedSet([]MemberParam{
					{value: "one", score: 1},
					{value: "two", score: 2},
					{value: "three", score: 3},
					{value: "four", score: 4},
				}),
				"key2": NewSortedSet([]MemberParam{
					{value: "three", score: 3},
					{value: "four", score: 4},
					{value: "five", score: 5},
					{value: "six", score: 6},
					{value: "seven", score: 7},
					{value: "eight", score: 8},
				}),
			},
			command:          []string{"ZDIFF", "key1", "key2", "WITHSCORES"},
			expectedResponse: []string{"one 1", "two 2"},
			expectedError:    nil,
		},
		{ // 3. Get the difference between 3 sets with scores.
			preset: true,
			presetValues: map[string]interface{}{
				"key3": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key4": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11},
				}),
				"key5": NewSortedSet([]MemberParam{
					{value: "seven", score: 7}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command:          []string{"ZDIFF", "key3", "key4", "key5", "WITHSCORES"},
			expectedResponse: []string{"three 3", "four 4", "five 5", "six 6"},
			expectedError:    nil,
		},
		{ // 3. Return sorted set if only one key exists and is a sorted set
			preset: true,
			presetValues: map[string]interface{}{
				"key6": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
			},
			command:          []string{"ZDIFF", "key6", "key7", "key8", "WITHSCORES"},
			expectedResponse: []string{"one 1", "two 2", "three 3", "four 4", "five 5", "six 6", "seven 7", "eight 8"},
			expectedError:    nil,
		},
		{ // 4. Throw error when one of the keys is not a sorted set.
			preset: true,
			presetValues: map[string]interface{}{
				"key9": "Default value",
				"key10": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11},
				}),
				"key11": NewSortedSet([]MemberParam{
					{value: "seven", score: 7}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command:          []string{"ZDIFF", "key9", "key10", "key11"},
			expectedResponse: nil,
			expectedError:    errors.New("value at key9 is not a sorted set"),
		},
		{ // 6. Command too short
			preset:           false,
			command:          []string{"ZDIFF"},
			expectedResponse: []string{},
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			for key, value := range test.presetValues {
				if _, err := mockServer.CreateKeyAndLock(context.Background(), key); err != nil {
					t.Error(err)
				}
				mockServer.SetValue(context.Background(), key, value)
				mockServer.KeyUnlock(key)
			}
		}
		res, err := handleZDIFF(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		if err != nil {
			t.Error(err)
		}
		rd := resp.NewReader(bytes.NewBuffer(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		for _, responseElement := range rv.Array() {
			if !slices.Contains(test.expectedResponse, responseElement.String()) {
				t.Errorf("could not find response element \"%s\" from expected response array", responseElement.String())
			}
		}
	}
}

func Test_HandleZDIFFSTORE(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValues     map[string]interface{}
		destination      string
		command          []string
		expectedValue    *SortedSet
		expectedResponse int
		expectedError    error
	}{
		{ // 1. Get the difference between 2 sorted sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key1": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5},
				}),
				"key2": NewSortedSet([]MemberParam{
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
			},
			destination:      "destination1",
			command:          []string{"ZDIFFSTORE", "destination1", "key1", "key2"},
			expectedValue:    NewSortedSet([]MemberParam{{value: "one", score: 1}, {value: "two", score: 2}}),
			expectedResponse: 2,
			expectedError:    nil,
		},
		{ // 2. Get the difference between 3 sorted sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key3": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key4": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11},
				}),
				"key5": NewSortedSet([]MemberParam{
					{value: "seven", score: 7}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			destination: "destination2",
			command:     []string{"ZDIFFSTORE", "destination2", "key3", "key4", "key5"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5}, {value: "six", score: 6},
			}),
			expectedResponse: 4,
			expectedError:    nil,
		},
		{ // 3. Return base sorted set element if base set is the only existing key provided and is a valid sorted set
			preset: true,
			presetValues: map[string]interface{}{
				"key6": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
			},
			destination: "destination3",
			command:     []string{"ZDIFFSTORE", "destination3", "key6", "key7", "key8"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "two", score: 2},
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5}, {value: "six", score: 6},
				{value: "seven", score: 7}, {value: "eight", score: 8},
			}),
			expectedResponse: 8,
			expectedError:    nil,
		},
		{ // 4. Throw error when base sorted set is not a set.
			preset: true,
			presetValues: map[string]interface{}{
				"key9": "Default value",
				"key10": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11},
				}),
				"key11": NewSortedSet([]MemberParam{
					{value: "seven", score: 7}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			destination:      "destination4",
			command:          []string{"ZDIFFSTORE", "destination4", "key9", "key10", "key11"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("value at key9 is not a sorted set"),
		},
		{ // 5. Throw error when base set is non-existent.
			preset:      true,
			destination: "destination5",
			presetValues: map[string]interface{}{
				"key12": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11},
				}),
				"key13": NewSortedSet([]MemberParam{
					{value: "seven", score: 7}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command:          []string{"ZDIFFSTORE", "destination5", "non-existent", "key12", "key13"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // 6. Command too short
			preset:           false,
			command:          []string{"ZDIFFSTORE", "destination6"},
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			for key, value := range test.presetValues {
				if _, err := mockServer.CreateKeyAndLock(context.Background(), key); err != nil {
					t.Error(err)
				}
				mockServer.SetValue(context.Background(), key, value)
				mockServer.KeyUnlock(key)
			}
		}
		res, err := handleZDIFFSTORE(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		if err != nil {
			t.Error(err)
		}
		rd := resp.NewReader(bytes.NewBuffer(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		if rv.Integer() != test.expectedResponse {
			t.Errorf("expected response integer %d, got %d", test.expectedResponse, rv.Integer())
		}
		if test.expectedValue != nil {
			if _, err = mockServer.KeyRLock(context.Background(), test.destination); err != nil {
				t.Error(err)
			}
			set, ok := mockServer.GetValue(test.destination).(*SortedSet)
			if !ok {
				t.Errorf("expected vaule at key %s to be set, got another type", test.destination)
			}
			for _, elem := range set.GetAll() {
				if !test.expectedValue.Contains(elem.value) {
					t.Errorf("could not find element %s in the expected values", elem.value)
				}
			}
			mockServer.KeyRUnlock(test.destination)
		}
	}
}

func Test_HandleZINCRBY(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValue      interface{}
		key              string
		command          []string
		expectedValue    *SortedSet
		expectedResponse string
		expectedError    error
	}{
		{ // 1. Successfully increment by int. Return the new score
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "two", score: 2},
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5},
			}),
			key:     "key1",
			command: []string{"ZINCRBY", "key1", "5", "one"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 6}, {value: "two", score: 2},
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5},
			}),
			expectedResponse: "6",
			expectedError:    nil,
		},
		{ // 2. Successfully increment by float. Return new score
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "two", score: 2},
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5},
			}),
			key:     "key2",
			command: []string{"ZINCRBY", "key2", "346.785", "one"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 347.785}, {value: "two", score: 2},
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5},
			}),
			expectedResponse: "347.785",
			expectedError:    nil,
		},
		{ // 3. Increment on non-existent sorted set will create the set with the member and increment as its score
			preset:      false,
			presetValue: nil,
			key:         "key3",
			command:     []string{"ZINCRBY", "key3", "346.785", "one"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 346.785},
			}),
			expectedResponse: "346.785",
			expectedError:    nil,
		},
		{ // 4. Increment score to +inf
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "two", score: 2},
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5},
			}),
			key:     "key4",
			command: []string{"ZINCRBY", "key4", "+inf", "one"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: Score(math.Inf(1))}, {value: "two", score: 2},
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5},
			}),
			expectedResponse: "+Inf",
			expectedError:    nil,
		},
		{ // 5. Increment score to -inf
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "two", score: 2},
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5},
			}),
			key:     "key5",
			command: []string{"ZINCRBY", "key5", "-inf", "one"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: Score(math.Inf(-1))}, {value: "two", score: 2},
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5},
			}),
			expectedResponse: "-Inf",
			expectedError:    nil,
		},
		{ // 6. Incrementing score by negative increment should lower the score
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "two", score: 2},
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5},
			}),
			key:     "key6",
			command: []string{"ZINCRBY", "key6", "-2.5", "five"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "two", score: 2},
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 2.5},
			}),
			expectedResponse: "2.5",
			expectedError:    nil,
		},
		{ // 7. Return error when attempting to increment on a value that is not a valid sorted set
			preset:           true,
			presetValue:      "Default value",
			key:              "key7",
			command:          []string{"ZINCRBY", "key7", "-2.5", "five"},
			expectedValue:    nil,
			expectedResponse: "",
			expectedError:    errors.New("value at key7 is not a sorted set"),
		},
		{ // 8. Return error when trying to increment a member that already has score -inf
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "one", score: Score(math.Inf(-1))},
			}),
			key:     "key8",
			command: []string{"ZINCRBY", "key8", "2.5", "one"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: Score(math.Inf(-1))},
			}),
			expectedResponse: "",
			expectedError:    errors.New("cannot increment -inf or +inf"),
		},
		{ // 9. Return error when trying to increment a member that already has score +inf
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "one", score: Score(math.Inf(1))},
			}),
			key:     "key9",
			command: []string{"ZINCRBY", "key9", "2.5", "one"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: Score(math.Inf(-1))},
			}),
			expectedResponse: "",
			expectedError:    errors.New("cannot increment -inf or +inf"),
		},
		{ // 10. Return error when increment is not a valid number
			preset: true,
			presetValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1},
			}),
			key:     "key10",
			command: []string{"ZINCRBY", "key10", "increment", "one"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1},
			}),
			expectedResponse: "",
			expectedError:    errors.New("increment must be a double"),
		},
		{ // 11. Command too short
			key:              "key11",
			command:          []string{"ZINCRBY", "key11", "one"},
			expectedResponse: "",
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
		{ // 12. Command too long
			key:              "key12",
			command:          []string{"ZINCRBY", "key12", "one", "1", "2"},
			expectedResponse: "",
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			if _, err := mockServer.CreateKeyAndLock(context.Background(), test.key); err != nil {
				t.Error(err)
			}
			mockServer.SetValue(context.Background(), test.key, test.presetValue)
			mockServer.KeyUnlock(test.key)
		}
		res, err := handleZINCRBY(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		if err != nil {
			t.Error(err)
		}
		rd := resp.NewReader(bytes.NewBuffer(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		if rv.String() != test.expectedResponse {
			t.Errorf("expected response integer %s, got %s", test.expectedResponse, rv.String())
		}
		if test.expectedValue != nil {
			if _, err = mockServer.KeyRLock(context.Background(), test.key); err != nil {
				t.Error(err)
			}
			set, ok := mockServer.GetValue(test.key).(*SortedSet)
			if !ok {
				t.Errorf("expected vaule at key %s to be set, got another type", test.key)
			}
			for _, elem := range set.GetAll() {
				if !test.expectedValue.Contains(elem.value) {
					t.Errorf("could not find element %s in the expected values", elem.value)
				}
				if test.expectedValue.Get(elem.value).score != elem.score {
					t.Errorf("expected score of element \"%s\" from set at key \"%s\" to be %s, got %s",
						elem.value, test.key,
						strconv.FormatFloat(float64(test.expectedValue.Get(elem.value).score), 'f', -1, 64),
						strconv.FormatFloat(float64(elem.score), 'f', -1, 64),
					)
				}
			}
			mockServer.KeyRUnlock(test.key)
		}
	}
}

func Test_HandleZINTER(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValues     map[string]interface{}
		command          []string
		expectedResponse []string
		expectedError    error
	}{
		{ // 1. Get the intersection between 2 sorted sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key1": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5},
				}),
				"key2": NewSortedSet([]MemberParam{
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
			},
			command:          []string{"ZINTER", "key1", "key2"},
			expectedResponse: []string{"three", "four", "five"},
			expectedError:    nil,
		},
		{
			// 2. Get the intersection between 3 sorted sets with scores.
			// By default, the SUM aggregate will be used.
			preset: true,
			presetValues: map[string]interface{}{
				"key3": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key4": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 8},
				}),
				"key5": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command:          []string{"ZINTER", "key3", "key4", "key5", "WITHSCORES"},
			expectedResponse: []string{"one 3", "eight 24"},
			expectedError:    nil,
		},
		{
			// 3. Get the intersection between 3 sorted sets with scores.
			// Use MIN aggregate.
			preset: true,
			presetValues: map[string]interface{}{
				"key6": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key7": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key8": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command:          []string{"ZINTER", "key6", "key7", "key8", "WITHSCORES", "AGGREGATE", "MIN"},
			expectedResponse: []string{"one 1", "eight 8"},
			expectedError:    nil,
		},
		{
			// 4. Get the intersection between 3 sorted sets with scores.
			// Use MAX aggregate.
			preset: true,
			presetValues: map[string]interface{}{
				"key9": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key10": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key11": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command:          []string{"ZINTER", "key9", "key10", "key11", "WITHSCORES", "AGGREGATE", "MAX"},
			expectedResponse: []string{"one 1000", "eight 800"},
			expectedError:    nil,
		},
		{
			// 5. Get the intersection between 3 sorted sets with scores.
			// Use SUM aggregate with weights modifier.
			preset: true,
			presetValues: map[string]interface{}{
				"key12": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key13": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key14": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command:          []string{"ZINTER", "key12", "key13", "key14", "WITHSCORES", "AGGREGATE", "SUM", "WEIGHTS", "1", "5", "3"},
			expectedResponse: []string{"one 3105", "eight 2808"},
			expectedError:    nil,
		},
		{
			// 6. Get the intersection between 3 sorted sets with scores.
			// Use MAX aggregate with added weights.
			preset: true,
			presetValues: map[string]interface{}{
				"key15": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key16": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key17": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command:          []string{"ZINTER", "key15", "key16", "key17", "WITHSCORES", "AGGREGATE", "MAX", "WEIGHTS", "1", "5", "3"},
			expectedResponse: []string{"one 3000", "eight 2400"},
			expectedError:    nil,
		},
		{
			// 7. Get the intersection between 3 sorted sets with scores.
			// Use MIN aggregate with added weights.
			preset: true,
			presetValues: map[string]interface{}{
				"key18": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key19": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key20": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command:          []string{"ZINTER", "key18", "key19", "key20", "WITHSCORES", "AGGREGATE", "MIN", "WEIGHTS", "1", "5", "3"},
			expectedResponse: []string{"one 5", "eight 8"},
			expectedError:    nil,
		},
		{ // 8. Throw an error if there are more weights than keys
			preset: true,
			presetValues: map[string]interface{}{
				"key21": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key22": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZINTER", "key21", "key22", "WEIGHTS", "1", "2", "3"},
			expectedResponse: nil,
			expectedError:    errors.New("number of weights should match number of keys"),
		},
		{ // 9. Throw an error if there are fewer weights than keys
			preset: true,
			presetValues: map[string]interface{}{
				"key23": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key24": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
				}),
				"key25": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZINTER", "key23", "key24", "key25", "WEIGHTS", "5", "4"},
			expectedResponse: nil,
			expectedError:    errors.New("number of weights should match number of keys"),
		},
		{ // 10. Throw an error if there are no keys provided
			preset: true,
			presetValues: map[string]interface{}{
				"key26": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
				"key27": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
				"key28": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZINTER", "WEIGHTS", "5", "4"},
			expectedResponse: nil,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
		{ // 11. Throw an error if any of the provided keys are not sorted sets
			preset: true,
			presetValues: map[string]interface{}{
				"key29": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key30": "Default value",
				"key31": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZINTER", "key29", "key30", "key31"},
			expectedResponse: nil,
			expectedError:    errors.New("value at key30 is not a sorted set"),
		},
		{ // 12. If any of the keys does not exist, return an empty array.
			preset: true,
			presetValues: map[string]interface{}{
				"key32": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11},
				}),
				"key33": NewSortedSet([]MemberParam{
					{value: "seven", score: 7}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command:          []string{"ZINTER", "non-existent", "key32", "key33"},
			expectedResponse: []string{},
			expectedError:    nil,
		},
		{ // 13. Command too short
			preset:           false,
			command:          []string{"ZINTER"},
			expectedResponse: []string{},
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			for key, value := range test.presetValues {
				if _, err := mockServer.CreateKeyAndLock(context.Background(), key); err != nil {
					t.Error(err)
				}
				mockServer.SetValue(context.Background(), key, value)
				mockServer.KeyUnlock(key)
			}
		}
		res, err := handleZINTER(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		if err != nil {
			t.Error(err)
		}
		rd := resp.NewReader(bytes.NewBuffer(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		for _, responseElement := range rv.Array() {
			if !slices.Contains(test.expectedResponse, responseElement.String()) {
				t.Errorf("could not find response element \"%s\" from expected response array", responseElement.String())
			}
		}
	}
}

func Test_HandleZINTERSTORE(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValues     map[string]interface{}
		destination      string
		command          []string
		expectedValue    *SortedSet
		expectedResponse int
		expectedError    error
	}{
		{ // 1. Get the intersection between 2 sorted sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key1": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5},
				}),
				"key2": NewSortedSet([]MemberParam{
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
			},
			destination: "destination1",
			command:     []string{"ZINTERSTORE", "destination1", "key1", "key2"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5},
			}),
			expectedResponse: 3,
			expectedError:    nil,
		},
		{
			// 2. Get the intersection between 3 sorted sets with scores.
			// By default, the SUM aggregate will be used.
			preset: true,
			presetValues: map[string]interface{}{
				"key3": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key4": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 8},
				}),
				"key5": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			destination: "destination2",
			command:     []string{"ZINTERSTORE", "destination2", "key3", "key4", "key5", "WITHSCORES"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "eight", score: 24},
			}),
			expectedResponse: 2,
			expectedError:    nil,
		},
		{
			// 3. Get the intersection between 3 sorted sets with scores.
			// Use MIN aggregate.
			preset: true,
			presetValues: map[string]interface{}{
				"key6": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key7": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key8": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			destination: "destination3",
			command:     []string{"ZINTERSTORE", "destination3", "key6", "key7", "key8", "WITHSCORES", "AGGREGATE", "MIN"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "eight", score: 8},
			}),
			expectedResponse: 2,
			expectedError:    nil,
		},
		{
			// 4. Get the intersection between 3 sorted sets with scores.
			// Use MAX aggregate.
			preset: true,
			presetValues: map[string]interface{}{
				"key9": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key10": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key11": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			destination: "destination4",
			command:     []string{"ZINTERSTORE", "destination4", "key9", "key10", "key11", "WITHSCORES", "AGGREGATE", "MAX"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1000}, {value: "eight", score: 800},
			}),
			expectedResponse: 2,
			expectedError:    nil,
		},
		{
			// 5. Get the intersection between 3 sorted sets with scores.
			// Use SUM aggregate with weights modifier.
			preset: true,
			presetValues: map[string]interface{}{
				"key12": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key13": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key14": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			destination: "destination5",
			command:     []string{"ZINTERSTORE", "destination5", "key12", "key13", "key14", "WITHSCORES", "AGGREGATE", "SUM", "WEIGHTS", "1", "5", "3"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "eight", score: 2808},
			}),
			expectedResponse: 2,
			expectedError:    nil,
		},
		{
			// 6. Get the intersection between 3 sorted sets with scores.
			// Use MAX aggregate with added weights.
			preset: true,
			presetValues: map[string]interface{}{
				"key15": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key16": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key17": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			destination: "destination6",
			command:     []string{"ZINTERSTORE", "destination6", "key15", "key16", "key17", "WITHSCORES", "AGGREGATE", "MAX", "WEIGHTS", "1", "5", "3"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 3000}, {value: "eight", score: 2400},
			}),
			expectedResponse: 2,
			expectedError:    nil,
		},
		{
			// 7. Get the intersection between 3 sorted sets with scores.
			// Use MIN aggregate with added weights.
			preset: true,
			presetValues: map[string]interface{}{
				"key18": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key19": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key20": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			destination: "destination7",
			command:     []string{"ZINTERSTORE", "destination7", "key18", "key19", "key20", "WITHSCORES", "AGGREGATE", "MIN", "WEIGHTS", "1", "5", "3"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 5}, {value: "eight", score: 8},
			}),
			expectedResponse: 2,
			expectedError:    nil,
		},
		{ // 8. Throw an error if there are more weights than keys
			preset: true,
			presetValues: map[string]interface{}{
				"key21": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key22": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZINTERSTORE", "destination8", "key21", "key22", "WEIGHTS", "1", "2", "3"},
			expectedResponse: 0,
			expectedError:    errors.New("number of weights should match number of keys"),
		},
		{ // 9. Throw an error if there are fewer weights than keys
			preset: true,
			presetValues: map[string]interface{}{
				"key23": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key24": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
				}),
				"key25": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZINTERSTORE", "destination9", "key23", "key24", "key25", "WEIGHTS", "5", "4"},
			expectedResponse: 0,
			expectedError:    errors.New("number of weights should match number of keys"),
		},
		{ // 10. Throw an error if there are no keys provided
			preset: true,
			presetValues: map[string]interface{}{
				"key26": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
				"key27": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
				"key28": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZINTERSTORE", "WEIGHTS", "5", "4"},
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
		{ // 11. Throw an error if any of the provided keys are not sorted sets
			preset: true,
			presetValues: map[string]interface{}{
				"key29": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key30": "Default value",
				"key31": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZINTERSTORE", "key29", "key30", "key31"},
			expectedResponse: 0,
			expectedError:    errors.New("value at key30 is not a sorted set"),
		},
		{ // 12. If any of the keys does not exist, return an empty array.
			preset: true,
			presetValues: map[string]interface{}{
				"key32": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11},
				}),
				"key33": NewSortedSet([]MemberParam{
					{value: "seven", score: 7}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command:          []string{"ZINTERSTORE", "destination12", "non-existent", "key32", "key33"},
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // 13. Command too short
			preset:           false,
			command:          []string{"ZINTERSTORE"},
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			for key, value := range test.presetValues {
				if _, err := mockServer.CreateKeyAndLock(context.Background(), key); err != nil {
					t.Error(err)
				}
				mockServer.SetValue(context.Background(), key, value)
				mockServer.KeyUnlock(key)
			}
		}
		res, err := handleZINTERSTORE(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		if err != nil {
			t.Error(err)
		}
		rd := resp.NewReader(bytes.NewBuffer(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		if rv.Integer() != test.expectedResponse {
			t.Errorf("expected response integer %d, got %d", test.expectedResponse, rv.Integer())
		}
		if test.expectedValue != nil {
			if _, err = mockServer.KeyRLock(context.Background(), test.destination); err != nil {
				t.Error(err)
			}
			set, ok := mockServer.GetValue(test.destination).(*SortedSet)
			if !ok {
				t.Errorf("expected vaule at key %s to be set, got another type", test.destination)
			}
			for _, elem := range set.GetAll() {
				if !test.expectedValue.Contains(elem.value) {
					t.Errorf("could not find element %s in the expected values", elem.value)
				}
			}
			mockServer.KeyRUnlock(test.destination)
		}
	}
}

func Test_HandleZMPOP(t *testing.T) {}

func Test_HandleZPOP(t *testing.T) {}

func Test_HandleZMSCORE(t *testing.T) {}

func Test_HandleZRANDMEMBER(t *testing.T) {}

func Test_HandleZRANK(t *testing.T) {}

func Test_HandleZREM(t *testing.T) {}

func Test_HandleZSCORE(t *testing.T) {}

func Test_HandleZREMRANGEBYSCORE(t *testing.T) {}

func Test_HandleZREMRANGEBYRANK(t *testing.T) {}

func Test_HandleZREMRANGEBYLEX(t *testing.T) {}

func Test_HandleZRANGE(t *testing.T) {}

func Test_HandleZRANGESTORE(t *testing.T) {}

func Test_HandleZUNION(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValues     map[string]interface{}
		command          []string
		expectedResponse []string
		expectedError    error
	}{
		{ // 1. Get the union between 2 sorted sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key1": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5},
				}),
				"key2": NewSortedSet([]MemberParam{
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
			},
			command:          []string{"ZUNION", "key1", "key2"},
			expectedResponse: []string{"one", "two", "three", "four", "five", "six", "seven", "eight"},
			expectedError:    nil,
		},
		{
			// 2. Get the union between 3 sorted sets with scores.
			// By default, the SUM aggregate will be used.
			preset: true,
			presetValues: map[string]interface{}{
				"key3": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key4": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 8},
				}),
				"key5": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12}, {value: "thirty-six", score: 36},
				}),
			},
			command: []string{"ZUNION", "key3", "key4", "key5", "WITHSCORES"},
			expectedResponse: []string{
				"one 3", "two 4", "three 3", "four 4", "five 5", "six 6",
				"seven 7", "eight 24", "nine 9", "ten 10", "eleven 11", "twelve 24", "thirty-six 72"},
			expectedError: nil,
		},
		{
			// 3. Get the union between 3 sorted sets with scores.
			// Use MIN aggregate.
			preset: true,
			presetValues: map[string]interface{}{
				"key6": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key7": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key8": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12}, {value: "thirty-six", score: 72},
				}),
			},
			command: []string{"ZUNION", "key6", "key7", "key8", "WITHSCORES", "AGGREGATE", "MIN"},
			expectedResponse: []string{
				"one 1", "two 2", "three 3", "four 4", "five 5", "six 6", "seven 7", "eight 8",
				"nine 9", "ten 10", "eleven 11", "twelve 12", "thirty-six 36",
			},
			expectedError: nil,
		},
		{
			// 4. Get the union between 3 sorted sets with scores.
			// Use MAX aggregate.
			preset: true,
			presetValues: map[string]interface{}{
				"key9": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key10": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key11": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12}, {value: "thirty-six", score: 72},
				}),
			},
			command: []string{"ZUNION", "key9", "key10", "key11", "WITHSCORES", "AGGREGATE", "MAX"},
			expectedResponse: []string{
				"one 1000", "two 2", "three 3", "four 4", "five 5", "six 6", "seven 7", "eight 800",
				"nine 9", "ten 10", "eleven 11", "twelve 12", "thirty-six 72",
			},
			expectedError: nil,
		},
		{
			// 5. Get the union between 3 sorted sets with scores.
			// Use SUM aggregate with weights modifier.
			preset: true,
			presetValues: map[string]interface{}{
				"key12": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key13": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key14": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command: []string{"ZUNION", "key12", "key13", "key14", "WITHSCORES", "AGGREGATE", "SUM", "WEIGHTS", "1", "2", "3"},
			expectedResponse: []string{
				"one 3102", "two 6", "three 3", "four 4", "five 5", "six 6", "seven 7", "eight 2568",
				"nine 27", "ten 30", "eleven 22", "twelve 60", "thirty-six 72",
			},
			expectedError: nil,
		},
		{
			// 6. Get the union between 3 sorted sets with scores.
			// Use MAX aggregate with added weights.
			preset: true,
			presetValues: map[string]interface{}{
				"key15": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key16": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key17": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command: []string{"ZUNION", "key15", "key16", "key17", "WITHSCORES", "AGGREGATE", "MAX", "WEIGHTS", "1", "2", "3"},
			expectedResponse: []string{
				"one 3000", "two 4", "three 3", "four 4", "five 5", "six 6", "seven 7", "eight 2400",
				"nine 27", "ten 30", "eleven 22", "twelve 36", "thirty-six 72",
			},
			expectedError: nil,
		},
		{
			// 7. Get the union between 3 sorted sets with scores.
			// Use MIN aggregate with added weights.
			preset: true,
			presetValues: map[string]interface{}{
				"key18": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key19": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key20": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command: []string{"ZUNION", "key18", "key19", "key20", "WITHSCORES", "AGGREGATE", "MIN", "WEIGHTS", "1", "2", "3"},
			expectedResponse: []string{
				"one 2", "two 2", "three 3", "four 4", "five 5", "six 6", "seven 7", "eight 8",
				"nine 27", "ten 30", "eleven 22", "twelve 24", "thirty-six 72",
			},
			expectedError: nil,
		},
		{ // 8. Throw an error if there are more weights than keys
			preset: true,
			presetValues: map[string]interface{}{
				"key21": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key22": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZUNION", "key21", "key22", "WEIGHTS", "1", "2", "3"},
			expectedResponse: nil,
			expectedError:    errors.New("number of weights should match number of keys"),
		},
		{ // 9. Throw an error if there are fewer weights than keys
			preset: true,
			presetValues: map[string]interface{}{
				"key23": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key24": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
				}),
				"key25": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZUNION", "key23", "key24", "key25", "WEIGHTS", "5", "4"},
			expectedResponse: nil,
			expectedError:    errors.New("number of weights should match number of keys"),
		},
		{ // 10. Throw an error if there are no keys provided
			preset: true,
			presetValues: map[string]interface{}{
				"key26": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
				"key27": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
				"key28": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZUNION", "WEIGHTS", "5", "4"},
			expectedResponse: nil,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
		{ // 11. Throw an error if any of the provided keys are not sorted sets
			preset: true,
			presetValues: map[string]interface{}{
				"key29": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key30": "Default value",
				"key31": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZUNION", "key29", "key30", "key31"},
			expectedResponse: nil,
			expectedError:    errors.New("value at key30 is not a sorted set"),
		},
		{ // 12. If any of the keys does not exist, skip it.
			preset: true,
			presetValues: map[string]interface{}{
				"key32": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11},
				}),
				"key33": NewSortedSet([]MemberParam{
					{value: "seven", score: 7}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			command: []string{"ZUNION", "non-existent", "key32", "key33"},
			expectedResponse: []string{
				"one", "two", "thirty-six", "twelve", "eleven",
				"seven", "eight", "nine", "ten",
			},
			expectedError: nil,
		},
		{ // 13. Command too short
			preset:           false,
			command:          []string{"ZUNION"},
			expectedResponse: []string{},
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			for key, value := range test.presetValues {
				if _, err := mockServer.CreateKeyAndLock(context.Background(), key); err != nil {
					t.Error(err)
				}
				mockServer.SetValue(context.Background(), key, value)
				mockServer.KeyUnlock(key)
			}
		}
		res, err := handleZUNION(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		if err != nil {
			t.Error(err)
		}
		rd := resp.NewReader(bytes.NewBuffer(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		for _, responseElement := range rv.Array() {
			if !slices.Contains(test.expectedResponse, responseElement.String()) {
				t.Errorf("could not find response element \"%s\" from expected response array", responseElement.String())
			}
		}
	}
}

func Test_HandleZUNIONSTORE(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValues     map[string]interface{}
		destination      string
		command          []string
		expectedValue    *SortedSet
		expectedResponse int
		expectedError    error
	}{
		{ // 1. Get the union between 2 sorted sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key1": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5},
				}),
				"key2": NewSortedSet([]MemberParam{
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
			},
			destination: "destination1",
			command:     []string{"ZUNIONSTORE", "destination1", "key1", "key2"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "two", score: 2},
				{value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5}, {value: "six", score: 6},
				{value: "seven", score: 7}, {value: "eight", score: 8},
			}),
			expectedResponse: 8,
			expectedError:    nil,
		},
		{
			// 2. Get the union between 3 sorted sets with scores.
			// By default, the SUM aggregate will be used.
			preset: true,
			presetValues: map[string]interface{}{
				"key3": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key4": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 8},
				}),
				"key5": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12}, {value: "thirty-six", score: 36},
				}),
			},
			destination: "destination2",
			command:     []string{"ZUNIONSTORE", "destination2", "key3", "key4", "key5", "WITHSCORES"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 3}, {value: "two", score: 4}, {value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5}, {value: "six", score: 6}, {value: "seven", score: 7}, {value: "eight", score: 24},
				{value: "nine", score: 9}, {value: "ten", score: 10}, {value: "eleven", score: 11},
				{value: "twelve", score: 24}, {value: "thirty-six", score: 72},
			}),
			expectedResponse: 13,
			expectedError:    nil,
		},
		{
			// 3. Get the union between 3 sorted sets with scores.
			// Use MIN aggregate.
			preset: true,
			presetValues: map[string]interface{}{
				"key6": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key7": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key8": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12}, {value: "thirty-six", score: 72},
				}),
			},
			destination: "destination3",
			command:     []string{"ZUNIONSTORE", "destination3", "key6", "key7", "key8", "WITHSCORES", "AGGREGATE", "MIN"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "two", score: 2}, {value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5}, {value: "six", score: 6}, {value: "seven", score: 7}, {value: "eight", score: 8},
				{value: "nine", score: 9}, {value: "ten", score: 10}, {value: "eleven", score: 11},
				{value: "twelve", score: 12}, {value: "thirty-six", score: 36},
			}),
			expectedResponse: 13,
			expectedError:    nil,
		},
		{
			// 4. Get the union between 3 sorted sets with scores.
			// Use MAX aggregate.
			preset: true,
			presetValues: map[string]interface{}{
				"key9": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key10": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key11": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12}, {value: "thirty-six", score: 72},
				}),
			},
			destination: "destination4",
			command: []string{
				"ZUNIONSTORE", "destination4", "key9", "key10", "key11", "WITHSCORES", "AGGREGATE", "MAX",
			},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1000}, {value: "two", score: 2}, {value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5}, {value: "six", score: 6}, {value: "seven", score: 7}, {value: "eight", score: 800},
				{value: "nine", score: 9}, {value: "ten", score: 10}, {value: "eleven", score: 11},
				{value: "twelve", score: 12}, {value: "thirty-six", score: 72},
			}),
			expectedResponse: 13,
			expectedError:    nil,
		},
		{
			// 5. Get the union between 3 sorted sets with scores.
			// Use SUM aggregate with weights modifier.
			preset: true,
			presetValues: map[string]interface{}{
				"key12": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key13": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key14": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			destination: "destination5",
			command: []string{
				"ZUNIONSTORE", "destination5", "key12", "key13", "key14",
				"WITHSCORES", "AGGREGATE", "SUM", "WEIGHTS", "1", "2", "3",
			},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 3102}, {value: "two", score: 6}, {value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5}, {value: "six", score: 6}, {value: "seven", score: 7}, {value: "eight", score: 2568},
				{value: "nine", score: 27}, {value: "ten", score: 30}, {value: "eleven", score: 22},
				{value: "twelve", score: 60}, {value: "thirty-six", score: 72},
			}),
			expectedResponse: 13,
			expectedError:    nil,
		},
		{
			// 6. Get the union between 3 sorted sets with scores.
			// Use MAX aggregate with added weights.
			preset: true,
			presetValues: map[string]interface{}{
				"key15": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key16": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key17": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			destination: "destination6",
			command: []string{
				"ZUNIONSTORE", "destination6", "key15", "key16", "key17",
				"WITHSCORES", "AGGREGATE", "MAX", "WEIGHTS", "1", "2", "3"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 3000}, {value: "two", score: 4}, {value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5}, {value: "six", score: 6}, {value: "seven", score: 7}, {value: "eight", score: 2400},
				{value: "nine", score: 27}, {value: "ten", score: 30}, {value: "eleven", score: 22},
				{value: "twelve", score: 36}, {value: "thirty-six", score: 72},
			}),
			expectedResponse: 13,
			expectedError:    nil,
		},
		{
			// 7. Get the union between 3 sorted sets with scores.
			// Use MIN aggregate with added weights.
			preset: true,
			presetValues: map[string]interface{}{
				"key18": NewSortedSet([]MemberParam{
					{value: "one", score: 100}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key19": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11}, {value: "eight", score: 80},
				}),
				"key20": NewSortedSet([]MemberParam{
					{value: "one", score: 1000}, {value: "eight", score: 800},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			destination: "destination7",
			command: []string{
				"ZUNIONSTORE", "destination7", "key18", "key19", "key20",
				"WITHSCORES", "AGGREGATE", "MIN", "WEIGHTS", "1", "2", "3",
			},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 2}, {value: "two", score: 2}, {value: "three", score: 3}, {value: "four", score: 4},
				{value: "five", score: 5}, {value: "six", score: 6}, {value: "seven", score: 7}, {value: "eight", score: 8},
				{value: "nine", score: 27}, {value: "ten", score: 30}, {value: "eleven", score: 22},
				{value: "twelve", score: 24}, {value: "thirty-six", score: 72},
			}),
			expectedResponse: 13,
			expectedError:    nil,
		},
		{ // 8. Throw an error if there are more weights than keys
			preset: true,
			presetValues: map[string]interface{}{
				"key21": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key22": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			destination:      "destination8",
			command:          []string{"ZUNIONSTORE", "destination8", "key21", "key22", "WEIGHTS", "1", "2", "3"},
			expectedResponse: 0,
			expectedError:    errors.New("number of weights should match number of keys"),
		},
		{ // 9. Throw an error if there are fewer weights than keys
			preset: true,
			presetValues: map[string]interface{}{
				"key23": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key24": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
				}),
				"key25": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			destination:      "destination9",
			command:          []string{"ZUNIONSTORE", "destination9", "key23", "key24", "key25", "WEIGHTS", "5", "4"},
			expectedResponse: 0,
			expectedError:    errors.New("number of weights should match number of keys"),
		},
		{ // 10. Throw an error if there are no keys provided
			preset: true,
			presetValues: map[string]interface{}{
				"key26": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
				"key27": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
				"key28": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			command:          []string{"ZUNIONSTORE", "WEIGHTS", "5", "4"},
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
		{ // 11. Throw an error if any of the provided keys are not sorted sets
			preset: true,
			presetValues: map[string]interface{}{
				"key29": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "three", score: 3}, {value: "four", score: 4},
					{value: "five", score: 5}, {value: "six", score: 6},
					{value: "seven", score: 7}, {value: "eight", score: 8},
				}),
				"key30": "Default value",
				"key31": NewSortedSet([]MemberParam{{value: "one", score: 1}}),
			},
			destination:      "destination11",
			command:          []string{"ZUNIONSTORE", "destination11", "key29", "key30", "key31"},
			expectedResponse: 0,
			expectedError:    errors.New("value at key30 is not a sorted set"),
		},
		{ // 12. If any of the keys does not exist, skip it.
			preset: true,
			presetValues: map[string]interface{}{
				"key32": NewSortedSet([]MemberParam{
					{value: "one", score: 1}, {value: "two", score: 2},
					{value: "thirty-six", score: 36}, {value: "twelve", score: 12},
					{value: "eleven", score: 11},
				}),
				"key33": NewSortedSet([]MemberParam{
					{value: "seven", score: 7}, {value: "eight", score: 8},
					{value: "nine", score: 9}, {value: "ten", score: 10},
					{value: "twelve", score: 12},
				}),
			},
			destination: "destination12",
			command:     []string{"ZUNIONSTORE", "destination12", "non-existent", "key32", "key33"},
			expectedValue: NewSortedSet([]MemberParam{
				{value: "one", score: 1}, {value: "two", score: 2}, {value: "seven", score: 7}, {value: "eight", score: 8},
				{value: "nine", score: 9}, {value: "ten", score: 10}, {value: "eleven", score: 11}, {value: "twelve", score: 12},
				{value: "thirty-six", score: 36},
			}),
			expectedResponse: 9,
			expectedError:    nil,
		},
		{ // 13. Command too short
			preset:           false,
			command:          []string{"ZUNIONSTORE"},
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			for key, value := range test.presetValues {
				if _, err := mockServer.CreateKeyAndLock(context.Background(), key); err != nil {
					t.Error(err)
				}
				mockServer.SetValue(context.Background(), key, value)
				mockServer.KeyUnlock(key)
			}
		}
		res, err := handleZUNIONSTORE(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		if err != nil {
			t.Error(err)
		}
		rd := resp.NewReader(bytes.NewBuffer(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		if rv.Integer() != test.expectedResponse {
			t.Errorf("expected response integer %d, got %d", test.expectedResponse, rv.Integer())
		}
		if test.expectedValue != nil {
			if _, err = mockServer.KeyRLock(context.Background(), test.destination); err != nil {
				t.Error(err)
			}
			set, ok := mockServer.GetValue(test.destination).(*SortedSet)
			if !ok {
				t.Errorf("expected vaule at key %s to be set, got another type", test.destination)
			}
			for _, elem := range set.GetAll() {
				if !test.expectedValue.Contains(elem.value) {
					t.Errorf("could not find element %s in the expected values", elem.value)
				}
			}
			mockServer.KeyRUnlock(test.destination)
		}
	}
}
