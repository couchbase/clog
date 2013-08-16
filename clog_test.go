package clog

import (
	"testing"
)

func TestLastComponent(t *testing.T) {
	tests := map[string]string{
		"plain":     "plain",
		"/a/b/c":    "c",
		"\\a\\b\\c": "c",
		"a/b/c":     "c",
		"a\\b\\c":   "c",
	}

	for in, exp := range tests {
		got := lastComponent(in)
		if got != exp {
			t.Errorf("Expected %q for %q, got %q", exp, in, got)
		}
	}
}
