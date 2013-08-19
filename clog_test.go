package clog

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
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

func TestParseLogFlags(t *testing.T) {
	defer SetOutput(os.Stderr)
	SetOutput(ioutil.Discard)
	ParseLogFlag("parsetest1,parsetest2+,bw,notime")
	exp := map[string]bool{"parsetest1": true, "parsetest1+": false,
		"parsetest2": true, "parsetest2+": true,
		"parsetest3": false,
		"bw":         false,
		"notime":     false}
	for k, v := range exp {
		if KeyEnabled(k) != v {
			t.Errorf("Expected %v enabled=%v, was %v",
				k, v, KeyEnabled(k))
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

func TestOutput(t *testing.T) {
	// reset the log when we're done
	defer SetOutput(os.Stderr)

	type niladic func()
	tests := []struct {
		f        niladic
		output   string
		exitVal  int
		panicked bool
	}{
		{
			func() {
				Log("testing %s", "123")
			},
			"testing 123", -1, false,
		},
		{
			func() {
				EnableKey("private")
				To("private", "testing %s", "123")
			},
			fgYellow + "private: " + reset + "testing 123", -1, false,
		},
		{
			func() {
				EnableKey("private")
				DisableKey("private")
				To("private", "testing %s", "123")
			},
			"", -1, false,
		},
		{
			func() {
				Printf("testing %s", "123")
			},
			"testing 123", -1, false,
		},
		{
			func() {
				Print("testing", "123")
			},
			"testing123", -1, false,
		},
		{
			func() {
				Error(fmt.Errorf("test error"))
			},
			fgRed + "ERROR: " + "test error" + reset, -1, false,
		},
		{
			func() {
				Warnf("testing %s", "123")
			},
			fgRed + "WARNING: " + "testing 123" + reset, -1, false,
		},
		{
			func() {
				Warn("testing", "123")
			},
			fgRed + "WARNING: " + "testing123" + reset, -1, false,
		},
		{
			func() {
				TEMPf("testing %s", "123")
			},
			fgYellow + "TEMP: " + "testing 123" + reset, -1, false,
		},
		{
			func() {
				TEMP("testing", "123")
			},
			fgYellow + "TEMP: " + "testing123" + reset, -1, false,
		},
		{
			func() {
				Fatal("testing", "123")
			},
			fgYellow + "FATAL: " + "testing123" + reset, 1, false,
		},
		{
			func() {
				Fatalf("testing12%d", 3)
			},
			fgYellow + "FATAL: " + "testing123" + reset, 1, false,
		},
		{
			func() {
				Panic("testing", "123")
			},
			fgYellow + "PANIC: " + "testing123" + reset, -1, true,
		},
		{
			func() {
				Panicf("testing12%d", 3)
			},
			fgYellow + "PANIC: " + "testing123" + reset, -1, true,
		},
	}

	exitVal := -1
	exit = func(i int) { exitVal = i }
	defer func() { exit = os.Exit }()

	for _, test := range tests {
		// reset our log buffer
		buffer := &bytes.Buffer{}
		SetOutput(buffer)
		// disable time so we can more easily compare
		DisableTime()
		exitVal = -1
		panicked := false
		func() {
			defer func() { panicked = recover() != nil }()
			test.f()
		}()
		if panicked != test.panicked {
			t.Errorf("Expected panic == %v, got %v", test.panicked, panicked)
		}
		if exitVal != test.exitVal {
			t.Errorf("Expected exitVal == %v, but got %v", test.exitVal, exitVal)
		}
		if buffer.Len() > 0 {
			usedBytes := buffer.Bytes()[0 : buffer.Len()-1]
			output := string(usedBytes)
			// strip off the caller info as we can't easily compare that
			callerLocation := strings.LastIndex(output, dim+" -- ")
			if callerLocation >= 0 {
				output = output[0:callerLocation]
			}
			if output != test.output {
				t.Errorf("Expected '%s' got '%s'", test.output, output)
			}
		} else {
			if test.output != "" {
				t.Errorf("Expected output `%s`, got none", test.output)
			}
		}
	}
}

func BenchmarkFlagLookupMiss(b *testing.B) {
	for i := 0; i < b.N; i++ {
		KeyEnabled("x")
	}
}

func BenchmarkFlagLookupHit(b *testing.B) {
	EnableKey("x")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		KeyEnabled("x")
	}
}

func BenchmarkFlagSet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		EnableKey("x")
	}
}

func BenchmarkToDisabled(b *testing.B) {
	defer SetOutput(os.Stderr)
	SetOutput(ioutil.Discard)
	DisableKey("btoe")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		To("btod", "thing")
	}
}

func BenchmarkToEnabled(b *testing.B) {
	defer SetOutput(os.Stderr)
	SetOutput(ioutil.Discard)
	EnableKey("btoe")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		To("btoe", "thing")
	}
}

func BenchmarkToWithFmt(b *testing.B) {
	defer SetOutput(os.Stderr)
	SetOutput(ioutil.Discard)
	EnableKey("btoe")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		To("btoe", "%s", "a string")
	}
}
