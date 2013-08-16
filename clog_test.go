package clog

import (
	"bytes"
	"fmt"
	"log"
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
	defer func() {
		logger = log.New(os.Stderr, "", log.Lmicroseconds)
	}()

	type niladic func()
	tests := []struct {
		f      niladic
		output string
	}{
		{
			func() {
				Log("testing %s", "123")
			},
			"testing 123",
		},
		{
			func() {
				EnableKey("private")
				To("private", "testing %s", "123")
			},
			fgYellow + "private: " + reset + "testing 123",
		},
		{
			func() {
				EnableKey("private")
				DisableKey("private")
				To("private", "testing %s", "123")
			},
			"",
		},
		{
			func() {
				Printf("testing %s", "123")
			},
			"testing 123",
		},
		{
			func() {
				Print("testing", "123")
			},
			"testing123",
		},
		{
			func() {
				Error(fmt.Errorf("test error"))
			},
			fgRed + "ERROR: " + "test error" + reset,
		},
		{
			func() {
				Warnf("testing %s", "123")
			},
			fgRed + "WARNING: " + "testing 123" + reset,
		},
		{
			func() {
				Warn("testing", "123")
			},
			fgRed + "WARNING: " + "testing123" + reset,
		},
		{
			func() {
				TEMPf("testing %s", "123")
			},
			fgYellow + "TEMP: " + "testing 123" + reset,
		},
		{
			func() {
				TEMP("testing", "123")
			},
			fgYellow + "TEMP: " + "testing123" + reset,
		},
	}

	for _, test := range tests {
		// reset our log buffer
		var buffer bytes.Buffer
		logger = log.New(&buffer, "", log.Lmicroseconds)
		// disable time so we can more easily compare
		NoTime()
		test.f()
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
	b.ResetTimer()
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EnableKey("x")
	}
}
