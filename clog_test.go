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

func TestGetCallersName(t *testing.T) {
	cn := getCallersName(0)
	if lastComponent(cn.filename) != "clog_test.go" {
		t.Errorf("Expected fn=clog_test.go, got %q",
			lastComponent(cn.filename))
	}
	if lastComponent(cn.funcname) != "clog.TestGetCallersName" {
		t.Errorf("Expected func=clog.TestGetCallersName, got %q",
			lastComponent(cn.funcname))
	}
	cn.String() // for side effect
	if cn = getCallersName(19); cn.String() != "???" {
		t.Errorf("Expected unknown call, got %q", cn.String())
	}
}

func TestKeyFlag(t *testing.T) {
	EnableKey("x")
	EnableKey("y")
	DisableKey("y")

	if !KeyEnabled("x") {
		t.Errorf("x should be enabled, but isn't")
	}
	if KeyEnabled("y") {
		t.Errorf("y should not be enabled, but is")
	}
	if KeyEnabled("z") {
		t.Errorf("z should not be enabled, but is")
	}
}
