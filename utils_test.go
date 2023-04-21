package storeit_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/feymanlee/storeit"
	"github.com/stretchr/testify/assert"
)

func TestAnyToString(t *testing.T) {
	var tests = []struct {
		name     string
		input    interface{}
		expected string
		err      error
	}{
		{"string", "hello", "hello", nil},
		{"int", 123, "123", nil},
		{"int8", int8(123), "123", nil},
		{"int16", int16(123), "123", nil},
		{"int32", int32(123), "123", nil},
		{"int64", int64(123), "123", nil},
		{"uint", uint(123), "123", nil},
		{"uint8", uint8(123), "123", nil},
		{"uint16", uint16(123), "123", nil},
		{"uint32", uint32(123), "123", nil},
		{"uint64", uint64(123), "123", nil},
		{"float32", float32(1.23), "1.23", nil},
		{"float64", 1.23, "1.23", nil},
		{"bool", true, "true", nil},
		{"[]byte", []byte("hello"), "hello", nil},
		{"time.Duration", 2 * time.Second, "2s", nil},
		{"json.Number", json.Number("123"), "123", nil},
		{"unknown type", complex(1, 2), "", errors.New("convert value type error")},
		{"nil", nil, "", nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			str, err := storeit.AnyToString(test.input)
			assert.Equal(t, test.expected, str)
			assert.Equal(t, test.err, err)
		})
	}
}

func TestAnyToInt(t *testing.T) {
	var tests = []struct {
		name     string
		input    interface{}
		expected int
		err      error
	}{
		{"int", 123, 123, nil},
		{"int8", int8(123), 123, nil},
		{"int16", int16(123), 123, nil},
		{"int32", int32(123), 123, nil},
		{"int64", int64(123), 123, nil},
		{"uint", uint(123), 123, nil},
		{"uint8", uint8(123), 123, nil},
		{"uint16", uint16(123), 123, nil},
		{"uint32", uint32(123), 123, nil},
		{"uint64", uint64(123), 123, nil},
		{"float32", float32(1.23), 1, nil},
		{"float64", float64(1.23), 1, nil},
		{"string", " 123 ", 123, nil},
		{"time.Duration", time.Second, int(time.Second), nil},
		{"json.Number", json.Number("123"), 123, nil},
		{"unknown type", complex(1, 2), 0, errors.New("convert value type error")},
		{"nil", nil, 0, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			iVal, err := storeit.AnyToInt(test.input)
			assert.Equal(t, test.expected, iVal)
			assert.Equal(t, test.err, err)
		})
	}
}
