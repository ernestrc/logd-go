package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	lua "github.com/Shopify/go-lua"
	"github.com/opentok/blue/logging"
)

type logWriter func(io.Writer, *logging.Log)

var validOutputModes = [1]string{"stdout"}
var validOutputFormats = [2]string{"JSON", "otlog"}

var script = flag.String("S", "", "Lua script that implements function log (time, level, flow, operation, step, properties). Defaults to printing a digest of the logs")
var scriptMode = flag.String("M", "bt", "Lua script load mode which controls whether the chunk can be text or binary (that is, a precompiled chunk). It may be the string 'b' (only binary chunks), 't' (only text chunks), or 'bt' (both binary and text). The default is 'bt'")
var outputFormat = flag.String("F", "otlog", "Output format to output fd. Default is 'otlog' which the same format than the input of the log. Other valid formats: 'JSON'")
var output = flag.String("O", "stdout", fmt.Sprintf("Output mode. Available modes: %v. Default is 'stdout'", validOutputModes))

// default lua script
var noop = "function log (timestamp, level, flow, operation, step, logptr)\nend"

func getOutput() (o io.Writer) {
	switch *output {
	case "stdout":
		o = stdoutLogWriter
	default:
		fmt.Printf("only %v are allowd as modes", validOutputModes)
		os.Exit(1)
	}
	return
}

func logWrite(o io.Writer, log *logging.Log) {
	o.Write([]byte(log.String()))
}

func jsonWrite(o io.Writer, log *logging.Log) {
	b := log.JSON()
	o.Write([]byte(b))
}

func getWriter() logWriter {
	switch *outputFormat {
	case "otlog":
		return logWrite
	case "JSON", "json":
		return jsonWrite
	default:
		fmt.Printf("invalid output format '%s'. Available: %v", *outputFormat, validOutputFormats)
		os.Exit(1)
	}

	return nil
}

func getArgLogPtr(l *lua.State, i int, fn string) *logging.Log {
	log, ok := l.ToUserData(i).(*logging.Log)
	if !ok {
		l.PushString(fmt.Sprintf("%d argument must be a pointer to a logging.Log structure in call to builtin '%s' function", i, fn))
		l.Error()
	}
	return log
}

func getArgString(l *lua.State, i int, fn string) string {
	key, ok := l.ToString(i)
	if !ok {
		l.PushString(fmt.Sprintf("%d argument must be a string in call to builtin '%s' function", i, fn))
		l.Error()
	}
	return key
}

// ResetLog resets all properties of a log to their zero values.
// Lua signature is function reset (logptr)
func ResetLog(l *lua.State) int {
	log := getArgLogPtr(l, 1, "reset")
	log.Reset()
	return 0
}

// RemoveProperty removes a property in the log with the given key. It returns true if it was removed;
// Lua signature is function remove (logptr, key)
func RemoveProperty(l *lua.State) int {
	log := getArgLogPtr(l, 1, "remove")
	key := getArgString(l, 2, "remove")
	l.PushBoolean(log.Remove(key))
	return 1
}

// SetProperty sets a property in the log to the given value.
// It returns true of the property was upserted and false if property was created.
// Lua signature is function set (logptr, key, value)
func SetProperty(l *lua.State) int {
	log := getArgLogPtr(l, 1, "set")
	key := getArgString(l, 2, "set")
	value := getArgString(l, 3, "set")
	l.PushBoolean(log.Set(key, value))
	return 1
}

// GetProperty returns the value of a property from the log or nil if log does not have property.
// It returns the property as a string or nil if log does not have property.
// Lua signature is function get (logptr, key)
func GetProperty(l *lua.State) (i int) {
	log := getArgLogPtr(l, 1, "get")
	key := getArgString(l, 2, "get")

	var value string
	var ok bool
	value, ok = log.Get(key)
	if !ok {
		l.PushNil()
		return 1
	}
	l.PushString(value)
	return 1
}

func loadUtils(l *lua.State) {
	l.PushGoFunction(GetProperty)
	l.SetGlobal("get")

	l.PushGoFunction(SetProperty)
	l.SetGlobal("set")

	l.PushGoFunction(RemoveProperty)
	l.SetGlobal("remove")
}

func loadLuaRuntime() *lua.State {
	l := lua.NewState()
	lua.OpenLibraries(l)
	loadUtils(l)

	var err error
	if *script == "" {
		err = lua.LoadString(l, noop)
	} else {
		err = lua.LoadFile(l, *script, *scriptMode)
	}
	if err != nil {
		fmt.Printf("there was an error when loading script: %s\n", err)
		os.Exit(1)
	}

	l.SetGlobal("log")
	return l
}

func main() {
	flag.Parse()

	p := logging.NewParser()
	reader := bufio.NewReader(os.Stdin)
	write := getWriter()
	writer := getOutput()
	logs := make([]logging.Log, 0)
	l := loadLuaRuntime()
	var buf [64 * 1000 * 1000]byte
	for {
		n, err := reader.Read(buf[:])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		logs = p.Parse(string(buf[:n]), logs)
		for _, log := range logs {
			l.Global("log")
			l.PushString(log.Timestamp())
			l.PushString(log.Level)
			l.PushString(log.Flow)
			l.PushString(log.Operation)
			l.PushString(log.Step)
			l.PushUserData(&log)
			l.Call(6, 0)
			write(writer, &log)
		}
		logs = logs[:0]
	}
}
