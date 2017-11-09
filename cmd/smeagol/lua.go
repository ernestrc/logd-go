package main

import (
	"fmt"
	"os"
	"reflect"

	lua "github.com/Shopify/go-lua"
	"github.com/opentok/blue/logging"
)

// default lua script
var noop = "function on_log (logptr)\nreturn logptr\nend"

const getLineQuery = "nSl"

func popLogPtr(l *lua.State, i int, fn string) *logging.Log {
	if !l.IsUserData(i) {
		// returning nil signals discarding the log
		if l.IsNil(i) {
			return nil
		}
		fmt.Fprintf(os.Stderr, "%d return value must be a pointer to a logging.Log structure in call to builtin '%s' function: found %s", i, fn, l.TypeOf(i))
		os.Exit(1)
	}
	ifc := l.ToUserData(i)
	l.Pop(i)

	log, ok := ifc.(*logging.Log)
	if !ok {
		fmt.Fprintf(os.Stderr, "%d return value must be a pointer to a logging.Log structure in call to builtin '%s' function: %+v", i, fn, reflect.TypeOf(ifc))
		os.Exit(1)
	}

	return log
}

func getArgLogPtr(l *lua.State, i int, fn string) *logging.Log {
	log, ok := l.ToUserData(i).(*logging.Log)
	if !ok {
		fmt.Fprintf(os.Stderr, "%d argument must be a pointer to a logging.Log structure in call to builtin '%s' function: found %s", i, fn, l.TypeOf(i))
		os.Exit(1)
	}
	return log
}

func getArgString(l *lua.State, i int, fn string) string {
	key, ok := l.ToString(i)
	if !ok {
		fmt.Fprintf(os.Stderr, "%d argument must be a string in call to builtin '%s' function: found %s", i, fn, l.TypeOf(i))
		os.Exit(1)
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

	l.PushGoFunction(ResetLog)
	l.SetGlobal("reset")
}

func loadScript(l *lua.State) {
	var err error
	if *script == "" {
		err = lua.DoString(l, noop)
	} else {
		err = lua.DoFile(l, *script)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "there was an error when loading script: %s\n", err)
		os.Exit(1)
	}
}

func loadLuaRuntime() *lua.State {
	l := lua.NewState()
	// TODO should whitelist libraries that are loaded
	lua.OpenLibraries(l)
	loadUtils(l)
	loadScript(l)
	return l
}

func callLuaMapFn(l *lua.State, log *logging.Log) *logging.Log {
	l.Global("on_log")
	if !l.IsFunction(-1) {
		panic("not defined in lua script: function on_log (logptr)")
	}
	l.PushUserData(log)
	l.Call(1, 1)
	ptr := popLogPtr(l, -1, "on_log")
	return ptr
}

func callLuaScheduledFn(l *lua.State) {
	l.Global("on_tick")
	if l.IsFunction(-1) {
		l.Call(0, 0)
	} else {
		l.Pop(-1)
	}
}
