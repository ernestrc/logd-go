package main

import (
	"fmt"
	"os"

	lua "github.com/Shopify/go-lua"
	"github.com/opentok/blue/logging"
)

// default lua script
var noop = "function map (logptr)\nreturn logptr\nend"

const getLineQuery = "nSl"

// bubble up a lua error which translates into a panic of the program
func luaPanic(l *lua.State, tmpl string, args ...interface{}) {
	f, ok := lua.Stack(l, 0)
	if !ok {
		panic("could not get lua script current frame")
	}
	var d lua.Debug
	if d, ok = lua.Info(l, getLineQuery, f); !ok {
		panic("could not get lua frame debug info")
	}

	err := fmt.Sprintf("%s: %+v", fmt.Sprintf(tmpl, args...), d)
	l.PushString(err)

	// panic
	l.Error()
}

func popLogPtr(l *lua.State, i int, fn string) *logging.Log {
	j := i * -1
	if !l.IsUserData(j) {
		// returning nil signals discarding the log
		if l.IsNil(j) {
			return nil
		}
		luaPanic(l, "%d return value must be a pointer to a logging.Log structure in call to builtin '%s' function: found %s", j, fn, l.TypeOf(i))
		return nil
	}
	ifc := l.ToUserData(j)
	l.Pop(i)

	log, ok := ifc.(*logging.Log)
	if !ok {
		luaPanic(l, "%d return value must be a pointer to a logging.Log structure in call to builtin '%s' function: %+v", j, fn, ifc)
	}

	return log
}

func getArgLogPtr(l *lua.State, i int, fn string) *logging.Log {
	log, ok := l.ToUserData(i).(*logging.Log)
	if !ok {
		luaPanic(l, "%d argument must be a pointer to a logging.Log structure in call to builtin '%s' function: found %s", i, fn, l.TypeOf(i))
	}
	return log
}

func getArgString(l *lua.State, i int, fn string) string {
	key, ok := l.ToString(i)
	if !ok {
		luaPanic(l, "%d argument must be a string in call to builtin '%s' function: found %s", i, fn, l.TypeOf(i))
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
		fmt.Fprintf(os.Stderr, "there was an error when loading script: %s\n", err)
		os.Exit(1)
	}

	l.SetGlobal("map")
	return l
}
