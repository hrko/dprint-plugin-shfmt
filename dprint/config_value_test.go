package dprint

import (
	"encoding/json"
	"testing"
)

func TestCoerceUInt32(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		expected uint32
		ok       bool
	}{
		{name: "uint32", value: uint32(2), expected: 2, ok: true},
		{name: "int64", value: int64(3), expected: 3, ok: true},
		{name: "float64", value: float64(4), expected: 4, ok: true},
		{name: "json-number", value: json.Number("5"), expected: 5, ok: true},
		{name: "string", value: "6", expected: 6, ok: true},
		{name: "bytes", value: []byte("7"), expected: 7, ok: true},
		{name: "negative", value: -1, expected: 0, ok: false},
		{name: "fraction", value: 1.5, expected: 0, ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value, ok := CoerceUInt32(tc.value)
			if ok != tc.ok {
				t.Fatalf("expected ok=%v, got %v", tc.ok, ok)
			}
			if value != tc.expected {
				t.Fatalf("expected value=%d, got %d", tc.expected, value)
			}
		})
	}
}

func TestCoerceBool(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		expected bool
		ok       bool
	}{
		{name: "bool", value: true, expected: true, ok: true},
		{name: "int-one", value: 1, expected: true, ok: true},
		{name: "int-zero", value: 0, expected: false, ok: true},
		{name: "json-number", value: json.Number("1"), expected: true, ok: true},
		{name: "string", value: "false", expected: false, ok: true},
		{name: "bytes", value: []byte("true"), expected: true, ok: true},
		{name: "invalid-int", value: 2, expected: false, ok: false},
		{name: "invalid-string", value: "yes", expected: false, ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value, ok := CoerceBool(tc.value)
			if ok != tc.ok {
				t.Fatalf("expected ok=%v, got %v", tc.ok, ok)
			}
			if value != tc.expected {
				t.Fatalf("expected value=%v, got %v", tc.expected, value)
			}
		})
	}
}
