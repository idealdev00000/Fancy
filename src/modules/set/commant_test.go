package set

import (
	"bytes"
	"context"
	"errors"
	"github.com/echovault/echovault/src/server"
	"github.com/echovault/echovault/src/utils"
	"github.com/tidwall/resp"
	"slices"
	"testing"
)

func Test_HandleSADD(t *testing.T) {
	mockserver := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValue      interface{}
		key              string
		command          []string
		expectedValue    *Set
		expectedResponse int
		expectedError    error
	}{
		{ // 1. Create new set on a non-existent key, return count of added elements
			preset:           false,
			presetValue:      nil,
			key:              "key1",
			command:          []string{"SADD", "key1", "one", "two", "three", "four"},
			expectedValue:    NewSet([]string{"one", "two", "three", "four"}),
			expectedResponse: 4,
			expectedError:    nil,
		},
		{ // 2. Add members to an exiting set, skip members that already exist in the set, return added count.
			preset:           true,
			presetValue:      NewSet([]string{"one", "two", "three", "four"}),
			key:              "key2",
			command:          []string{"SADD", "key2", "three", "four", "five", "six", "seven"},
			expectedValue:    NewSet([]string{"one", "two", "three", "four", "five", "six", "seven"}),
			expectedResponse: 3,
			expectedError:    nil,
		},
		{ // 3. Throw error when trying to add to a key that does not hold a set
			preset:           true,
			presetValue:      "Default value",
			key:              "key3",
			command:          []string{"SADD", "key3", "member"},
			expectedResponse: 0,
			expectedError:    errors.New("value at key key3 is not a set"),
		},
		{ // 4. Command too short
			preset:           false,
			key:              "key4",
			command:          []string{"SADD", "key4"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			if _, err := mockserver.CreateKeyAndLock(context.Background(), test.key); err != nil {
				t.Error(err)
			}
			mockserver.SetValue(context.Background(), test.key, test.presetValue)
			mockserver.KeyUnlock(test.key)
		}
		res, err := handleSADD(context.Background(), test.command, mockserver, nil)
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
			t.Errorf("expected integer response %d, got %d", test.expectedResponse, rv.Integer())
		}
		if _, err = mockserver.KeyRLock(context.Background(), test.key); err != nil {
			t.Error(err)
		}
		set, ok := mockserver.GetValue(test.key).(*Set)
		if !ok {
			t.Errorf("expected set value at key \"%s\"", test.key)
		}
		if set.Cardinality() != test.expectedValue.Cardinality() {
			t.Errorf("expected resulting cardinality to be %d, got %d", test.expectedValue.Cardinality(), set.Cardinality())
		}
		for _, member := range set.GetAll() {
			if !test.expectedValue.Contains(member) {
				t.Errorf("could not find member \"%s\" in expected set", member)
			}
		}
		mockserver.KeyRUnlock(test.key)
	}
}

func Test_HandleSCARD(t *testing.T) {
	mockserver := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValue      interface{}
		key              string
		command          []string
		expectedValue    *Set
		expectedResponse int
		expectedError    error
	}{
		{ // 1. Get cardinality of valid set.
			preset:           true,
			presetValue:      NewSet([]string{"one", "two", "three", "four"}),
			key:              "key1",
			command:          []string{"SCARD", "key1"},
			expectedValue:    nil,
			expectedResponse: 4,
			expectedError:    nil,
		},
		{ // 2. Return 0 when trying to get cardinality on non-existent key
			preset:           false,
			presetValue:      nil,
			key:              "key2",
			command:          []string{"SCARD", "key2"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // 3. Throw error when trying to get cardinality of a value that is not a set
			preset:           true,
			presetValue:      "Default value",
			key:              "key3",
			command:          []string{"SCARD", "key3"},
			expectedResponse: 0,
			expectedError:    errors.New("value at key key3 is not a set"),
		},
		{ // 4. Command too short
			preset:           false,
			key:              "key4",
			command:          []string{"SCARD"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
		{ // 5. Command too long
			preset:           false,
			key:              "key5",
			command:          []string{"SCARD", "key5", "key5"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			if _, err := mockserver.CreateKeyAndLock(context.Background(), test.key); err != nil {
				t.Error(err)
			}
			mockserver.SetValue(context.Background(), test.key, test.presetValue)
			mockserver.KeyUnlock(test.key)
		}
		res, err := handleSCARD(context.Background(), test.command, mockserver, nil)
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
			t.Errorf("expected integer response %d, got %d", test.expectedResponse, rv.Integer())
		}
	}
}

func Test_HandleSDIFF(t *testing.T) {
	mockserver := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValues     map[string]interface{}
		command          []string
		expectedResponse []string
		expectedError    error
	}{
		{ // 1. Get the difference between 2 sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key1": NewSet([]string{"one", "two", "three", "four", "five"}),
				"key2": NewSet([]string{"three", "four", "five", "six", "seven", "eight"}),
			},
			command:          []string{"SDIFF", "key1", "key2"},
			expectedResponse: []string{"one", "two"},
			expectedError:    nil,
		},
		{ // 2. Get the difference between 3 sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key3": NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}),
				"key4": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven"}),
				"key5": NewSet([]string{"seven", "eight", "nine", "ten", "twelve"}),
			},
			command:          []string{"SDIFF", "key3", "key4", "key5"},
			expectedResponse: []string{"three", "four", "five", "six"},
			expectedError:    nil,
		},
		{ // 3. Return base set element if base set is the only valid set
			preset: true,
			presetValues: map[string]interface{}{
				"key6": NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}),
				"key7": "Default value",
				"key8": 123456789,
			},
			command:          []string{"SDIFF", "key6", "key7", "key8"},
			expectedResponse: []string{"one", "two", "three", "four", "five", "six", "seven", "eight"},
			expectedError:    nil,
		},
		{ // 4. Throw error when base set is not a set.
			preset: true,
			presetValues: map[string]interface{}{
				"key9":  "Default value",
				"key10": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven"}),
				"key11": NewSet([]string{"seven", "eight", "nine", "ten", "twelve"}),
			},
			command:          []string{"SDIFF", "key9", "key10", "key11"},
			expectedResponse: nil,
			expectedError:    errors.New("value at key key9 is not a set"),
		},
		{ // 5. Throw error when base set is non-existent.
			preset: true,
			presetValues: map[string]interface{}{
				"key12": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven"}),
				"key13": NewSet([]string{"seven", "eight", "nine", "ten", "twelve"}),
			},
			command:          []string{"SDIFF", "non-existent", "key7", "key8"},
			expectedResponse: nil,
			expectedError:    errors.New("key for base set \"non-existent\" does not exist"),
		},
		{ // 6. Command too short
			preset:           false,
			command:          []string{"SDIFF"},
			expectedResponse: []string{},
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			for key, value := range test.presetValues {
				if _, err := mockserver.CreateKeyAndLock(context.Background(), key); err != nil {
					t.Error(err)
				}
				mockserver.SetValue(context.Background(), key, value)
				mockserver.KeyUnlock(key)
			}
		}
		res, err := handleSDIFF(context.Background(), test.command, mockserver, nil)
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

func Test_HandleSDIFFSTORE(t *testing.T) {
	mockserver := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValues     map[string]interface{}
		destination      string
		command          []string
		expectedValue    *Set
		expectedResponse int
		expectedError    error
	}{
		{ // 1. Get the difference between 2 sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key1": NewSet([]string{"one", "two", "three", "four", "five"}),
				"key2": NewSet([]string{"three", "four", "five", "six", "seven", "eight"}),
			},
			destination:      "destination1",
			command:          []string{"SDIFFSTORE", "destination1", "key1", "key2"},
			expectedValue:    NewSet([]string{"one", "two"}),
			expectedResponse: 2,
			expectedError:    nil,
		},
		{ // 2. Get the difference between 3 sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key3": NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}),
				"key4": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven"}),
				"key5": NewSet([]string{"seven", "eight", "nine", "ten", "twelve"}),
			},
			destination:      "destination2",
			command:          []string{"SDIFFSTORE", "destination2", "key3", "key4", "key5"},
			expectedValue:    NewSet([]string{"three", "four", "five", "six"}),
			expectedResponse: 4,
			expectedError:    nil,
		},
		{ // 3. Return base set element if base set is the only valid set
			preset: true,
			presetValues: map[string]interface{}{
				"key6": NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}),
				"key7": "Default value",
				"key8": 123456789,
			},
			destination:      "destination3",
			command:          []string{"SDIFFSTORE", "destination3", "key6", "key7", "key8"},
			expectedValue:    NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}),
			expectedResponse: 8,
			expectedError:    nil,
		},
		{ // 4. Throw error when base set is not a set.
			preset: true,
			presetValues: map[string]interface{}{
				"key9":  "Default value",
				"key10": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven"}),
				"key11": NewSet([]string{"seven", "eight", "nine", "ten", "twelve"}),
			},
			destination:      "destination4",
			command:          []string{"SDIFFSTORE", "destination4", "key9", "key10", "key11"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("value at key key9 is not a set"),
		},
		{ // 5. Throw error when base set is non-existent.
			preset:      true,
			destination: "destination5",
			presetValues: map[string]interface{}{
				"key12": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven"}),
				"key13": NewSet([]string{"seven", "eight", "nine", "ten", "twelve"}),
			},
			command:          []string{"SDIFFSTORE", "destination5", "non-existent", "key7", "key8"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("key for base set \"non-existent\" does not exist"),
		},
		{ // 6. Command too short
			preset:           false,
			command:          []string{"SDIFFSTORE", "destination6"},
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			for key, value := range test.presetValues {
				if _, err := mockserver.CreateKeyAndLock(context.Background(), key); err != nil {
					t.Error(err)
				}
				mockserver.SetValue(context.Background(), key, value)
				mockserver.KeyUnlock(key)
			}
		}
		res, err := handleSDIFFSTORE(context.Background(), test.command, mockserver, nil)
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
			if _, err = mockserver.KeyRLock(context.Background(), test.destination); err != nil {
				t.Error(err)
			}
			set, ok := mockserver.GetValue(test.destination).(*Set)
			if !ok {
				t.Errorf("expected vaule at key %s to be set, got another type", test.destination)
			}
			for _, elem := range set.GetAll() {
				if !test.expectedValue.Contains(elem) {
					t.Errorf("could not find element %s in the expected values", elem)
				}
			}
			mockserver.KeyRUnlock(test.destination)
		}
	}
}

func Test_HandleSINTER(t *testing.T) {
	mockserver := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValues     map[string]interface{}
		command          []string
		expectedResponse []string
		expectedError    error
	}{
		{ // 1. Get the intersection between 2 sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key1": NewSet([]string{"one", "two", "three", "four", "five"}),
				"key2": NewSet([]string{"three", "four", "five", "six", "seven", "eight"}),
			},
			command:          []string{"SINTER", "key1", "key2"},
			expectedResponse: []string{"three", "four", "five"},
			expectedError:    nil,
		},
		{ // 2. Get the difference between 3 sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key3": NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}),
				"key4": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven", "eight"}),
				"key5": NewSet([]string{"one", "eight", "nine", "ten", "twelve"}),
			},
			command:          []string{"SINTER", "key3", "key4", "key5"},
			expectedResponse: []string{"one", "eight"},
			expectedError:    nil,
		},
		{ // 3. Throw an error if any of the provided keys are not sets
			preset: true,
			presetValues: map[string]interface{}{
				"key6": NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}),
				"key7": "Default value",
				"key8": NewSet([]string{"one"}),
			},
			command:          []string{"SINTER", "key6", "key7", "key8"},
			expectedResponse: nil,
			expectedError:    errors.New("value at key key7 is not a set"),
		},
		{ // 4. Throw error when base set is not a set.
			preset: true,
			presetValues: map[string]interface{}{
				"key9":  "Default value",
				"key10": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven"}),
				"key11": NewSet([]string{"seven", "eight", "nine", "ten", "twelve"}),
			},
			command:          []string{"SINTER", "key9", "key10", "key11"},
			expectedResponse: nil,
			expectedError:    errors.New("value at key key9 is not a set"),
		},
		{ // 5. If any of the keys does not exist, return an empty array.
			preset: true,
			presetValues: map[string]interface{}{
				"key12": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven"}),
				"key13": NewSet([]string{"seven", "eight", "nine", "ten", "twelve"}),
			},
			command:          []string{"SINTER", "non-existent", "key7", "key8"},
			expectedResponse: []string{},
			expectedError:    nil,
		},
		{ // 6. Command too short
			preset:           false,
			command:          []string{"SINTER"},
			expectedResponse: []string{},
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			for key, value := range test.presetValues {
				if _, err := mockserver.CreateKeyAndLock(context.Background(), key); err != nil {
					t.Error(err)
				}
				mockserver.SetValue(context.Background(), key, value)
				mockserver.KeyUnlock(key)
			}
		}
		res, err := handleSINTER(context.Background(), test.command, mockserver, nil)
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

func Test_HandleSINTERCARD(t *testing.T) {
	mockserver := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValues     map[string]interface{}
		command          []string
		expectedResponse int
		expectedError    error
	}{
		{ // 1. Get the full intersect cardinality between 2 sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key1": NewSet([]string{"one", "two", "three", "four", "five"}),
				"key2": NewSet([]string{"three", "four", "five", "six", "seven", "eight"}),
			},
			command:          []string{"SINTERCARD", "key1", "key2"},
			expectedResponse: 3,
			expectedError:    nil,
		},
		{ // 2. Get an intersect cardinality between 2 sets with a limit
			preset: true,
			presetValues: map[string]interface{}{
				"key3": NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}),
				"key4": NewSet([]string{"three", "four", "five", "six", "seven", "eight", "nine", "ten", "eleven", "twelve"}),
			},
			command:          []string{"SINTERCARD", "key3", "key4", "LIMIT", "3"},
			expectedResponse: 3,
			expectedError:    nil,
		},
		{ // 3. Get the full intersect cardinality between 3 sets.
			preset: true,
			presetValues: map[string]interface{}{
				"key5": NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}),
				"key6": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven", "eight"}),
				"key7": NewSet([]string{"one", "seven", "eight", "nine", "ten", "twelve"}),
			},
			command:          []string{"SINTERCARD", "key5", "key6", "key7"},
			expectedResponse: 2,
			expectedError:    nil,
		},
		{ // 4. Get the intersection of 3 sets with a limit
			preset: true,
			presetValues: map[string]interface{}{
				"key8":  NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}),
				"key9":  NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven", "eight"}),
				"key10": NewSet([]string{"one", "two", "seven", "eight", "nine", "ten", "twelve"}),
			},
			command:          []string{"SINTERCARD", "key8", "key9", "key10", "LIMIT", "2"},
			expectedResponse: 2,
			expectedError:    nil,
		},
		{ // 5. Return 0 if any of the keys does not exist
			preset: true,
			presetValues: map[string]interface{}{
				"key11": NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}),
				"key12": "Default value",
				"key13": NewSet([]string{"one"}),
			},
			command:          []string{"SINTERCARD", "key11", "key12", "key13", "non-existent"},
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // 6. Throw error when one of the keys is not a valid set.
			preset: true,
			presetValues: map[string]interface{}{
				"key14": "Default value",
				"key15": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven"}),
				"key16": NewSet([]string{"seven", "eight", "nine", "ten", "twelve"}),
			},
			command:          []string{"SINTERSTORE", "key14", "key15", "key16"},
			expectedResponse: 0,
			expectedError:    errors.New("value at key key14 is not a set"),
		},
		{ // 7. Command too short
			preset:           false,
			command:          []string{"SINTERSTORE"},
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			for key, value := range test.presetValues {
				if _, err := mockserver.CreateKeyAndLock(context.Background(), key); err != nil {
					t.Error(err)
				}
				mockserver.SetValue(context.Background(), key, value)
				mockserver.KeyUnlock(key)
			}
		}
		res, err := handleSINTERCARD(context.Background(), test.command, mockserver, nil)
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
	}
}

func Test_HandleSINTERSTORE(t *testing.T) {
	mockserver := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		presetValues     map[string]interface{}
		destination      string
		command          []string
		expectedValue    *Set
		expectedResponse int
		expectedError    error
	}{
		{ // 1. Get the intersection between 2 sets and store it at the destination.
			preset: true,
			presetValues: map[string]interface{}{
				"key1": NewSet([]string{"one", "two", "three", "four", "five"}),
				"key2": NewSet([]string{"three", "four", "five", "six", "seven", "eight"}),
			},
			destination:      "destination1",
			command:          []string{"SINTERSTORE", "destination1", "key1", "key2"},
			expectedValue:    NewSet([]string{"three", "four", "five"}),
			expectedResponse: 3,
			expectedError:    nil,
		},
		{ // 2. Get the intersection between 3 sets and store it at the destination key.
			preset: true,
			presetValues: map[string]interface{}{
				"key3": NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}),
				"key4": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven", "eight"}),
				"key5": NewSet([]string{"one", "seven", "eight", "nine", "ten", "twelve"}),
			},
			destination:      "destination2",
			command:          []string{"SINTERSTORE", "destination2", "key3", "key4", "key5"},
			expectedValue:    NewSet([]string{"one", "eight"}),
			expectedResponse: 2,
			expectedError:    nil,
		},
		{ // 3. Throw error when any of the keys is not a set
			preset: true,
			presetValues: map[string]interface{}{
				"key6": NewSet([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}),
				"key7": "Default value",
				"key8": NewSet([]string{"one"}),
			},
			destination:      "destination3",
			command:          []string{"SINTERSTORE", "destination3", "key6", "key7", "key8"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("value at key key7 is not a set"),
		},
		{ // 4. Throw error when base set is not a set.
			preset: true,
			presetValues: map[string]interface{}{
				"key9":  "Default value",
				"key10": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven"}),
				"key11": NewSet([]string{"seven", "eight", "nine", "ten", "twelve"}),
			},
			destination:      "destination4",
			command:          []string{"SINTERSTORE", "destination4", "key9", "key10", "key11"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    errors.New("value at key key9 is not a set"),
		},
		{ // 5. Return an empty intersection if one of the keys does not exist.
			preset:      true,
			destination: "destination5",
			presetValues: map[string]interface{}{
				"key12": NewSet([]string{"one", "two", "thirty-six", "twelve", "eleven"}),
				"key13": NewSet([]string{"seven", "eight", "nine", "ten", "twelve"}),
			},
			command:          []string{"SINTERSTORE", "destination5", "non-existent", "key7", "key8"},
			expectedValue:    nil,
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // 6. Command too short
			preset:           false,
			command:          []string{"SINTERSTORE", "destination6"},
			expectedResponse: 0,
			expectedError:    errors.New(utils.WRONG_ARGS_RESPONSE),
		},
	}

	for _, test := range tests {
		if test.preset {
			for key, value := range test.presetValues {
				if _, err := mockserver.CreateKeyAndLock(context.Background(), key); err != nil {
					t.Error(err)
				}
				mockserver.SetValue(context.Background(), key, value)
				mockserver.KeyUnlock(key)
			}
		}
		res, err := handleSINTERSTORE(context.Background(), test.command, mockserver, nil)
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
			if _, err = mockserver.KeyRLock(context.Background(), test.destination); err != nil {
				t.Error(err)
			}
			set, ok := mockserver.GetValue(test.destination).(*Set)
			if !ok {
				t.Errorf("expected vaule at key %s to be set, got another type", test.destination)
			}
			for _, elem := range set.GetAll() {
				if !test.expectedValue.Contains(elem) {
					t.Errorf("could not find element %s in the expected values", elem)
				}
			}
			mockserver.KeyRUnlock(test.destination)
		}
	}
}

func Test_HandleSISMEMBER(t *testing.T) {}

func Test_HandleSMEMBERS(t *testing.T) {}

func Test_HandleSMOVE(t *testing.T) {}

func Test_HandleSPOP(t *testing.T) {}

func Test_HandleSRANDMEMBER(t *testing.T) {}

func Test_HandleSREM(t *testing.T) {}

func Test_HandleSUNION(t *testing.T) {}

func Test_HandleSUNIONSTORE(t *testing.T) {}
