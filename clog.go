//  Copyright (c) 2012-2013 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package clog

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"unsafe"
)

// Log level type
type logLevel int

const (
	LevelNormal = logLevel(iota)
	LevelWarning
	LevelPanic
)

// Logging package level
var Level = LevelNormal

// Set of To() key strings that are enabled.
var keys unsafe.Pointer = unsafe.Pointer(&map[string]bool{})

var logger *log.Logger = log.New(os.Stderr, "", log.Lmicroseconds)

// Disables ANSI color in log output.
func DisableColor() {
	reset, dim, fgRed, fgYellow = "", "", "", ""
}

// Disable timestamps in logs.
func DisableTime() {
	logger.SetFlags(logger.Flags() &^ (log.Ldate | log.Ltime | log.Lmicroseconds))
}

// SetOutput sets the output destination for clog
func SetOutput(w io.Writer) {
	logger = log.New(w, "", logger.Flags())
}

// Parses a comma-separated list of log keys, probably coming from an argv flag.
// The key "bw" is interpreted as a call to NoColor, not a key.
func ParseLogFlag(flag string) {
	if flag != "" {
		ParseLogFlags(strings.Split(flag, ","))
	}
}

// Parses an array of log keys, probably coming from a argv flags.
// The key "bw" is interpreted as a call to NoColor, not a key.
func ParseLogFlags(flags []string) {
	for _, key := range flags {
		switch key {
		case "bw":
			DisableColor()
		case "notime":
			DisableTime()
		default:
			EnableKey(key)
			for strings.HasSuffix(key, "+") {
				key = key[0 : len(key)-1]
				EnableKey(key) // "foo+" also enables "foo"
			}
		}
	}
	Log("Enabling logging: %s", flags)
}

// Enable logging messages sent to this key
func EnableKey(key string) {
	for {
		opp := atomic.LoadPointer(&keys)
		oldk := (*map[string]bool)(opp)
		newk := map[string]bool{key: true}
		for k := range *oldk {
			newk[k] = true
		}
		if atomic.CompareAndSwapPointer(&keys, opp, unsafe.Pointer(&newk)) {
			return
		}
	}
}

// Disable logging messages sent to this key
func DisableKey(key string) {
	for {
		opp := atomic.LoadPointer(&keys)
		oldk := (*map[string]bool)(opp)
		newk := map[string]bool{}
		for k := range *oldk {
			if k != key {
				newk[k] = true
			}
		}
		if atomic.CompareAndSwapPointer(&keys, opp, unsafe.Pointer(&newk)) {
			return
		}
	}
}

// Check to see if logging is enabled for a key
func KeyEnabled(key string) bool {
	m := *(*map[string]bool)(atomic.LoadPointer(&keys))
	return m[key]
}

type callInfo struct {
	funcname, filename string
	line               int
}

func (c callInfo) String() string {
	if c.funcname == "" {
		return "???"
	}
	return fmt.Sprintf("%s() at %s:%d", lastComponent(c.funcname),
		lastComponent(c.filename), c.line)
}

// Returns a string identifying a function on the call stack.
// Use depth=1 for the caller of the function that calls GetCallersName, etc.
func getCallersName(depth int) callInfo {
	pc, file, line, ok := runtime.Caller(depth + 1)
	if !ok {
		return callInfo{}
	}

	fnname := ""
	if fn := runtime.FuncForPC(pc); fn != nil {
		fnname = fn.Name()
	}

	return callInfo{fnname, file, line}
}

// Logs a message to the console, but only if the corresponding key is true in keys.
func To(key string, format string, args ...interface{}) {
	if Level <= LevelNormal && KeyEnabled(key) {
		logger.Printf(fgYellow+key+": "+reset+format, args...)
	}
}

// Logs a message to the console.
func Log(format string, args ...interface{}) {
	if Level <= LevelNormal {
		logger.Printf(format, args...)
	}
}

// Prints a formatted message to the console.
func Printf(format string, args ...interface{}) {
	if Level <= LevelNormal {
		logger.Printf(format, args...)
	}
}

// Prints a message to the console.
func Print(args ...interface{}) {
	if Level <= LevelNormal {
		logger.Print(args...)
	}
}

// If the error is not nil, logs its description and the name of the calling function.
// Returns the input error for easy chaining.
func Error(err error) error {
	if Level <= LevelWarning && err != nil {
		logWithCallerf(fgRed, "ERROR", "%v", err)
	}
	return err
}

// Logs a formatted warning to the console
func Warnf(format string, args ...interface{}) {
	if Level <= LevelWarning {
		logWithCallerf(fgRed, "WARNING", format, args...)
	}
}

// Logs a warning to the console
func Warn(args ...interface{}) {
	if Level <= LevelWarning {
		logWithCaller(fgRed, "WARNING", args...)
	}
}

// Logs a highlighted message prefixed with "TEMP". This function is intended for
// temporary logging calls added during development and not to be checked in, hence its
// distinctive name (which is visible and easy to search for before committing.)
func TEMPf(format string, args ...interface{}) {
	logWithCallerf(fgYellow, "TEMP", format, args...)
}

// Logs a highlighted message prefixed with "TEMP". This function is intended for
// temporary logging calls added during development and not to be checked in, hence its
// distinctive name (which is visible and easy to search for before committing.)
func TEMP(args ...interface{}) {
	logWithCaller(fgYellow, "TEMP", args...)
}

// Logs a formatted warning to the console, then panics.
func Panicf(format string, args ...interface{}) {
	logWithCallerf(fgRed, "PANIC", format, args...)
	panic(fmt.Sprintf(format, args...))
}

// Logs a warning to the console, then panics.
func Panic(args ...interface{}) {
	logWithCaller(fgRed, "PANIC", args...)
	panic(fmt.Sprint(args...))
}

// Logs a formatted warning to the console, then exits the process.
func Fatalf(format string, args ...interface{}) {
	logWithCallerf(fgRed, "FATAL", format, args...)
	os.Exit(1)
}

// Logs a warning to the console, then exits the process.
func Fatal(args ...interface{}) {
	logWithCaller(fgRed, "FATAL", args...)
	os.Exit(1)
}

func logWithCaller(color string, prefix string, args ...interface{}) {
	message := fmt.Sprint(args...)
	logger.Print(color, prefix, ": ", message, reset,
		dim, " -- ", getCallersName(2), reset)
}

func logWithCallerf(color string, prefix string, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	logger.Print(color, prefix, ": ", message, reset,
		dim, " -- ", getCallersName(2), reset)
}

func lastComponent(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		path = path[index+1:]
	} else if index = strings.LastIndex(path, "\\"); index >= 0 {
		path = path[index+1:]
	}
	return path
}

// ANSI color control escape sequences.
// Shamelessly copied from https://github.com/sqp/godock/blob/master/libs/log/colors.go
var (
	reset      = "\x1b[0m"
	bright     = "\x1b[1m"
	dim        = "\x1b[2m"
	underscore = "\x1b[4m"
	blink      = "\x1b[5m"
	reverse    = "\x1b[7m"
	hidden     = "\x1b[8m"
	fgBlack    = "\x1b[30m"
	fgRed      = "\x1b[31m"
	fgGreen    = "\x1b[32m"
	fgYellow   = "\x1b[33m"
	fgBlue     = "\x1b[34m"
	fgMagenta  = "\x1b[35m"
	fgCyan     = "\x1b[36m"
	fgWhite    = "\x1b[37m"
	bgBlack    = "\x1b[40m"
	bgRed      = "\x1b[41m"
	bgGreen    = "\x1b[42m"
	bgYellow   = "\x1b[43m"
	bgBlue     = "\x1b[44m"
	bgMagenta  = "\x1b[45m"
	bgCyan     = "\x1b[46m"
	bgWhite    = "\x1b[47m"
)
