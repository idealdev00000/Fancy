package str

import (
	"bytes"
	"context"
	"errors"
	"github.com/echovault/echovault/src/server"
	"github.com/echovault/echovault/src/utils"
	"github.com/tidwall/resp"
	"strconv"
	"testing"
)

func Test_HandleSetRange(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		key              string
		presetValue      string
		command          []string
		expectedValue    string
		expectedResponse int
		expectedError    error
	}{
		{ // Test that SETRANGE on non-existent string creates new string
			preset:           false,
			key:              "test1",
			presetValue:      "",
			command:          []string{"SETRANGE", "test1", "10", "New String Value"},
			expectedValue:    "New String Value",
			expectedResponse: len("New String Value"),
			expectedError:    nil,
		},
		{ // Test SETRANGE with an offset that leads to a longer resulting string
			preset:           true,
			key:              "test2",
			presetValue:      "Original String Value",
			command:          []string{"SETRANGE", "test2", "16", "Portion Replaced With This New String"},
			expectedValue:    "Original String Portion Replaced With This New String",
			expectedResponse: len("Original String Portion Replaced With This New String"),
			expectedError:    nil,
		},
		{ // SETRANGE with negative offset prepends the string
			preset:           true,
			key:              "test3",
			presetValue:      "This is a preset value",
			command:          []string{"SETRANGE", "test3", "-10", "Prepended "},
			expectedValue:    "Prepended This is a preset value",
			expectedResponse: len("Prepended This is a preset value"),
			expectedError:    nil,
		},
		{ // SETRANGE with offset that embeds new string inside the old string
			preset:           true,
			key:              "test4",
			presetValue:      "This is a preset value",
			command:          []string{"SETRANGE", "test4", "0", "That"},
			expectedValue:    "That is a preset value",
			expectedResponse: len("That is a preset value"),
			expectedError:    nil,
		},
		{ // SETRANGE with offset longer than original lengths appends the string
			preset:           true,
			key:              "test5",
			presetValue:      "This is a preset value",
			command:          []string{"SETRANGE", "test5", "100", " Appended"},
			expectedValue:    "This is a preset value Appended",
			expectedResponse: len("This is a preset value Appended"),
			expectedError:    nil,
		},
		{ // SETRANGE with offset on the last character replaces last character with new string
			preset:           true,
			key:              "test6",
			presetValue:      "This is a preset value",
			command:          []string{"SETRANGE", "test6", strconv.Itoa(len("This is a preset value") - 1), " replaced"},
			expectedValue:    "This is a preset valu replaced",
			expectedResponse: len("This is a preset valu replaced"),
			expectedError:    nil,
		},
		{ // Offset not integer
			preset:           false,
			command:          []string{"SETRANGE", "key", "offset", "value"},
			expectedResponse: 0,
			expectedError:    errors.New("offset must be an integer"),
		},
		{ // SETRANGE target is not a string
			preset:           true,
			key:              "test-int",
			presetValue:      "10",
			command:          []string{"SETRANGE", "test-int", "10", "value"},
			expectedResponse: 0,
			expectedError:    errors.New("value at key test-int is not a string"),
		},
		{ // Command too short
			preset:           false,
			command:          []string{"SETRANGE", "key"},
			expectedResponse: 0,
			expectedError:    errors.New(utils.WrongArgsResponse),
		},
		{ // Command too long
			preset:           false,
			command:          []string{"SETRANGE", "key", "offset", "value", "value1"},
			expectedResponse: 0,
			expectedError:    errors.New(utils.WrongArgsResponse),
		},
	}

	for _, test := range tests {
		// If there's a preset step, carry it out here
		if test.preset {
			if _, err := mockServer.CreateKeyAndLock(context.Background(), test.key); err != nil {
				t.Error(err)
			}
			mockServer.SetValue(context.Background(), test.key, utils.AdaptType(test.presetValue))
			mockServer.KeyUnlock(test.key)
		}

		res, err := handleSetRange(context.Background(), test.command, mockServer, nil)
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
			t.Errorf("expected response \"%d\", got \"%d\"", test.expectedResponse, rv.Integer())
		}

		// Get the value from the server and check against the expected value
		if _, err = mockServer.KeyRLock(context.Background(), test.key); err != nil {
			t.Error(err)
		}
		value, ok := mockServer.GetValue(context.Background(), test.key).(string)
		if !ok {
			t.Error("expected string data type, got another type")
		}
		if value != test.expectedValue {
			t.Errorf("expected value \"%s\", got \"%s\"", test.expectedValue, value)
		}
		mockServer.KeyRUnlock(test.key)
	}
}

func Test_HandleStrLen(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		key              string
		presetValue      string
		command          []string
		expectedResponse int
		expectedError    error
	}{
		{ // Return the correct string length for an existing string
			preset:           true,
			key:              "test1",
			presetValue:      "Test String",
			command:          []string{"STRLEN", "test1"},
			expectedResponse: len("Test String"),
			expectedError:    nil,
		},
		{ // If the string does not exist, return 0
			preset:           false,
			key:              "test2",
			presetValue:      "",
			command:          []string{"STRLEN", "test2"},
			expectedResponse: 0,
			expectedError:    nil,
		},
		{ // Too few args
			preset:           false,
			key:              "test3",
			presetValue:      "",
			command:          []string{"STRLEN"},
			expectedResponse: 0,
			expectedError:    errors.New(utils.WrongArgsResponse),
		},
		{ // Too many args
			preset:           false,
			key:              "test4",
			presetValue:      "",
			command:          []string{"STRLEN", "test4", "test5"},
			expectedResponse: 0,
			expectedError:    errors.New(utils.WrongArgsResponse),
		},
	}

	for _, test := range tests {
		if test.preset {
			_, err := mockServer.CreateKeyAndLock(context.Background(), test.key)
			if err != nil {
				t.Error(err)
			}
			mockServer.SetValue(context.Background(), test.key, test.presetValue)
			mockServer.KeyUnlock(test.key)
		}
		res, err := handleStrLen(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		rd := resp.NewReader(bytes.NewBuffer(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		if rv.Integer() != test.expectedResponse {
			t.Errorf("expected respons \"%d\", got \"%d\"", test.expectedResponse, rv.Integer())
		}
	}
}

func Test_HandleSubStr(t *testing.T) {
	mockServer := server.NewServer(server.Opts{})

	tests := []struct {
		preset           bool
		key              string
		presetValue      string
		command          []string
		expectedResponse string
		expectedError    error
	}{
		{ // Return substring within the range of the string
			preset:           true,
			key:              "test1",
			presetValue:      "Test String One",
			command:          []string{"SUBSTR", "test1", "5", "10"},
			expectedResponse: "String",
			expectedError:    nil,
		},
		{ // Return substring at the end of the string with exact end index
			preset:           true,
			key:              "test2",
			presetValue:      "Test String Two",
			command:          []string{"SUBSTR", "test2", "12", "14"},
			expectedResponse: "Two",
			expectedError:    nil,
		},
		{ // Return substring at the end of the string with end index greater than length
			preset:           true,
			key:              "test3",
			presetValue:      "Test String Three",
			command:          []string{"SUBSTR", "test3", "12", "75"},
			expectedResponse: "Three",
			expectedError:    nil,
		},
		{ // Return the substring at the start of the string with 0 start index
			preset:           true,
			key:              "test4",
			presetValue:      "Test String Four",
			command:          []string{"SUBSTR", "test4", "0", "3"},
			expectedResponse: "Test",
			expectedError:    nil,
		},
		{
			// Return the substring with negative start index.
			// Substring should begin abs(start) from the end of the string when start is negative.
			preset:           true,
			key:              "test5",
			presetValue:      "Test String Five",
			command:          []string{"SUBSTR", "test5", "-11", "10"},
			expectedResponse: "String",
			expectedError:    nil,
		},
		{
			// Return reverse substring with end index smaller than start index.
			// When end index is smaller than start index, the 2 indices are reversed.
			preset:           true,
			key:              "test6",
			presetValue:      "Test String Six",
			command:          []string{"SUBSTR", "test6", "4", "0"},
			expectedResponse: "tseT",
			expectedError:    nil,
		},
		{ // Command too short
			command:       []string{"SUBSTR", "key", "10"},
			expectedError: errors.New(utils.WrongArgsResponse),
		},
		{
			// Command too long
			command:       []string{"SUBSTR", "key", "10", "15", "20"},
			expectedError: errors.New(utils.WrongArgsResponse),
		},
		{ // Start index is not an integer
			command:       []string{"SUBSTR", "key", "start", "10"},
			expectedError: errors.New("start and end indices must be integers"),
		},
		{ // End index is not an integer
			command:       []string{"SUBSTR", "key", "0", "end"},
			expectedError: errors.New("start and end indices must be integers"),
		},
		{ // Non-existent key
			command:       []string{"SUBSTR", "non-existent-key", "0", "10"},
			expectedError: errors.New("key non-existent-key does not exist"),
		},
	}

	for _, test := range tests {
		if test.preset {
			_, err := mockServer.CreateKeyAndLock(context.Background(), test.key)
			if err != nil {
				t.Error(err)
			}
			mockServer.SetValue(context.Background(), test.key, test.presetValue)
			mockServer.KeyUnlock(test.key)
		}
		res, err := handleSubStr(context.Background(), test.command, mockServer, nil)
		if test.expectedError != nil {
			if err.Error() != test.expectedError.Error() {
				t.Errorf("expected error \"%s\", got \"%s\"", test.expectedError.Error(), err.Error())
			}
			continue
		}
		rd := resp.NewReader(bytes.NewBuffer(res))
		rv, _, err := rd.ReadValue()
		if err != nil {
			t.Error(err)
		}
		if rv.String() != test.expectedResponse {
			t.Errorf("expected response \"%s\", got \"%s\"", test.expectedResponse, rv.String())
		}
	}
}
