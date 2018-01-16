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

// Log level type.
type LogLevel int32

const (
	LevelDebug = LogLevel(iota)
	LevelNormal
	LevelWarning
	LevelError
	LevelPanic
)

// Logging package level (Setting Level directly isn't thread-safe).
var Level = LevelNormal

// Should caller be included in log messages (stored as 0 or 1 to enable thread-safe access)
var includeCaller = int32(1)

// Set of To() key strings that are enabled.
var keys unsafe.Pointer = unsafe.Pointer(&map[string]bool{})

var logger *log.Logger = log.New(os.Stderr, "", log.LstdFlags)
var logCallBack func(level, format string, args ...interface{}) string

// Thread-safe API for setting log level.
func SetLevel(to LogLevel) {
	for {
		if atomic.CompareAndSwapInt32((*int32)(&Level),
			int32(GetLevel()), int32(to)) {
			break
		}
	}
}

// Thread-safe API for fetching log level.
func GetLevel() LogLevel {
	return LogLevel(atomic.LoadInt32((*int32)(&Level)))
}

// Thread-safe API for configuring whether caller information is included in log output. (default true)
func SetIncludeCaller(enabled bool) {
	for {
		if atomic.CompareAndSwapInt32((*int32)(&includeCaller),
			btoi(IsIncludeCaller()), btoi(enabled)) {
			break
		}
	}
}

// Thread-safe API for indicating whether caller information is included in log output.
func IsIncludeCaller() bool {
	return atomic.LoadInt32((*int32)(&includeCaller)) == 1
}

// Category that the data falls under.
type ContentCategory int32

const (
	UserData = ContentCategory(iota)
	MetaData
	SystemData
	numTypes // This is to always be the last entry.
)

var tags []string

func init() {
	tags = []string{
		"ud", // Couchbase UserData
		"md", // Couchbase MetaData
		"sd", // Couchbase SystemData
	}
}

func Tag(category ContentCategory, data interface{}) interface{} {
	if category < numTypes {
		return fmt.Sprintf("<%s>%s</%s>", tags[category], data, tags[category])
	}

	return data
}

// Flags returns the output flags for clog.
func Flags() int {
	return logger.Flags()
}

// SetFlags sets the output flags for clog.
func SetFlags(flags int) {
	logger.SetFlags(flags)
}

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
	ParseLogFlags(strings.Split(flag, ","))
}

// Set a prefix function for the log message. Prefix function is called for
// each log message and it returns a prefix which is logged before each message
func SetLoggerCallback(k func(level, format string, args ...interface{}) string) {
	// Clear the date and time flag
	DisableTime()
	logCallBack = k
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
				key = key[:len(key)-1]
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
	if GetLevel() <= LevelNormal && KeyEnabled(key) {
		if logCallBack != nil {
			str := logCallBack("INFO", format, args...)
			if str != "" {
				logger.Print(str)
			}
		} else {
			logger.Printf(fgYellow+key+": "+reset+format, args...)
		}
	}
}

// Logs a message to the console.
func Log(format string, args ...interface{}) {
	if GetLevel() <= LevelNormal {
		if logCallBack != nil {
			str := logCallBack("INFO", format, args...)
			if str != "" {
				logger.Print(str)
			}
		} else {
			logger.Printf(format, args...)
		}
	}
}

// Prints a formatted message to the console.
func Printf(format string, args ...interface{}) {
	if GetLevel() <= LevelNormal {
		if logCallBack != nil {
			str := logCallBack("INFO", format, args...)
			if str != "" {
				logger.Print(str)
			}
		} else {
			logger.Printf(format, args...)
		}
	}
}

// Prints a message to the console.
func Print(args ...interface{}) {
	if GetLevel() <= LevelNormal {
		if logCallBack != nil {
			str := logCallBack("INFO", "", args...)
			if str != "" {
				logger.Print(str)
			}
		} else {
			logger.Print(args...)
		}
	}
}

// If the error is not nil, logs error to the console. Returns the input error
// for easy chaining.
func Error(err error) error {
	if GetLevel() <= LevelError && err != nil {
		doLogf(fgRed, "ERRO", "%v", err)
	}
	return err
}

// Logs a formatted error message to the console
func Errorf(format string, args ...interface{}) {
	if GetLevel() <= LevelError {
		doLogf(fgRed, "ERRO", format, args...)
	}
}

// Logs a formatted warning to the console
func Warnf(format string, args ...interface{}) {
	if GetLevel() <= LevelWarning {
		doLogf(fgRed, "WARN", format, args...)
	}
}

// Logs a warning to the console
func Warn(args ...interface{}) {
	if GetLevel() <= LevelWarning {
		doLog(fgRed, "WARN", args...)
	}
}

// Logs a formatted debug message to the console
func Debugf(format string, args ...interface{}) {
	if GetLevel() <= LevelDebug {
		doLogf(fgRed, "DEBU", format, args...)
	}
}

// Logs a debug message to the console
func Debug(args ...interface{}) {
	if GetLevel() <= LevelDebug {
		doLog(fgRed, "DEBU", args...)
	}
}

// Logs a highlighted message prefixed with "TEMP". This function is intended for
// temporary logging calls added during development and not to be checked in, hence its
// distinctive name (which is visible and easy to search for before committing.)
func TEMPf(format string, args ...interface{}) {
	doLogf(fgYellow, "TEMP", format, args...)
}

// Logs a highlighted message prefixed with "TEMP". This function is intended for
// temporary logging calls added during development and not to be checked in, hence its
// distinctive name (which is visible and easy to search for before committing.)
func TEMP(args ...interface{}) {
	doLog(fgYellow, "TEMP", args...)
}

// Logs a formatted warning to the console, then panics.
func Panicf(format string, args ...interface{}) {
	doLogf(fgRed, "CRIT", format, args...)
	panic(fmt.Sprintf(format, args...))
}

// Logs a warning to the console, then panics.
func Panic(args ...interface{}) {
	doLog(fgRed, "CRIT", args...)
	panic(fmt.Sprint(args...))
}

// For test fixture
var exit = os.Exit

// Logs a formatted warning to the console, then exits the process.
func Fatalf(format string, args ...interface{}) {
	doLogf(fgRed, "FATA", format, args...)
	exit(1)
}

// Logs a warning to the console, then exits the process.
func Fatal(args ...interface{}) {
	doLog(fgRed, "FATA", args...)
	exit(1)
}

func doLog(color string, prefix string, args ...interface{}) {
	message := fmt.Sprint(args...)
	if logCallBack != nil {
		str := logCallBack(prefix, "", args...)
		if str != "" {
			if IsIncludeCaller() {
				logger.Print(str, " -- ", getCallersName(2))
			} else {
				logger.Print(str)
			}
		}
	} else {
		if IsIncludeCaller() {
			logger.Print(color, prefix, ": ", message, reset,
				dim, " -- ", getCallersName(2), reset)
		} else {
			logger.Print(color, prefix, ": ", message, reset, dim)
		}
	}
}

func doLogf(color string, prefix string, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	if logCallBack != nil {
		str := logCallBack(prefix, format, args...)
		if str != "" {
			if IsIncludeCaller() {
				logger.Print(str, " -- ", getCallersName(2))
			} else {
				logger.Print(str)
			}
		}
	} else {
		if IsIncludeCaller() {
			logger.Print(color, prefix, ": ", message, reset,
				dim, " -- ", getCallersName(2), reset)
		} else {
			logger.Print(color, prefix, ": ", message, reset, dim)
		}
	}
}

func lastComponent(path string) string {
	if index := strings.LastIndex(path, "/"); index >= 0 {
		path = path[index+1:]
	} else if index = strings.LastIndex(path, "\\"); index >= 0 {
		path = path[index+1:]
	}
	return path
}

func btoi(boolean bool) int32 {
	if boolean {
		return 1
	}
	return 0
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
