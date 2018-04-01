package lua

import (
	"fmt"

	lua "github.com/Shopify/go-lua"
	"github.com/ernestrc/logd/logging"
	log "github.com/sirupsen/logrus"
)

const (
	luaNameConfigFn       = "config_set"
	luaNameHTTPGetFn      = "http_get"
	luaNameHTTPPostFn     = "http_post"
	luaNameKafkaProduceFn = "kafka_produce"
	luaNameKafkaOffsetFn  = "kafka_offset"
	luaNameKafkaMessageFn = "kafka_message"
	luaNameGetFn          = "log_get"
	luaNameSetFn          = "log_set"
	luaNameRemoveFn       = "log_remove"
	luaNameResetFn        = "log_reset"
	luaNameLogStringFn    = "log_string"
	luaNameLogJSONFn      = "log_json"
	luaNameDebugFn        = "debug"
)

var logdAPI = []lua.RegistryFunction{
	/* module API */
	{Name: luaNameHTTPGetFn, Function: luaHTTPGet},
	{Name: luaNameHTTPPostFn, Function: luaHTTPPost},
	{Name: luaNameConfigFn, Function: luaSetConfig},
	{Name: luaNameGetFn, Function: luaGetLogProperty},
	{Name: luaNameSetFn, Function: luaSetLogProperty},
	{Name: luaNameRemoveFn, Function: luaRemoveLogProperty},
	{Name: luaNameResetFn, Function: luaResetLog},
	{Name: luaNameLogStringFn, Function: luaLogString},
	{Name: luaNameLogJSONFn, Function: luaLogJSON},
	{Name: luaNameKafkaOffsetFn, Function: luaKafkaOffset},
	{Name: luaNameKafkaMessageFn, Function: luaKafkaMessage},
	{Name: luaNameKafkaProduceFn, Function: luaKafkaProduce},
	{Name: luaNameDebugFn, Function: luaDebug},
	/* hooks are left undefined
	{Name: luaNameOnLogFn, Function: nil},
	{Name: luaNameOnTickFn, Function: nil},
	{Name: luaNameOnHTTPErrorFn, Function: nil},
	{Name: luaNameOnKafkaReportFn, Function: nil},
	*/
}

func getArgLogPtr(l *lua.State, i int, fn string) *logging.Log {
	log, ok := l.ToUserData(i).(*logging.Log)
	if !ok {
		panic(fmt.Errorf(
			"%d argument must be a pointer to a Log structure in call to builtin '%s' function: found %s",
			i, fn, l.TypeOf(i)))
	}
	return log
}

func getArgString(l *lua.State, i int, fn string) string {
	arg, ok := l.ToString(i)
	if !ok {
		panic(fmt.Errorf(
			"%d argument must be a string in call to builtin '%s' function: found %s",
			i, fn, l.TypeOf(i)))
	}
	return arg
}

func getOptionalArgInt(l *lua.State, i, def int, fn string) int {
	arg, ok := l.ToInteger(i)
	if !ok {
		return def
	}
	return arg
}

func getArgBool(l *lua.State, i int, fn string) bool {
	arg := l.ToBoolean(i)
	if !l.IsBoolean(i) {
		panic(fmt.Errorf(
			"%d argument must be a boolean in call to builtin '%s' function: found %s",
			i, fn, l.TypeOf(i)))
	}
	return arg
}

func getArgInt(l *lua.State, i int, fn string) int {
	arg, ok := l.ToInteger(i)
	if !ok {
		panic(fmt.Errorf(
			"%d argument must be an integer in call to builtin '%s' function: found %s",
			i, fn, l.TypeOf(i)))
	}
	return arg
}

func getStateSandbox(l *lua.State) *Sandbox {
	l.Global(luaNameSandboxContext)
	sandbox, ok := l.ToUserData(-1).(*Sandbox)
	if !ok {
		panic(fmt.Errorf("corrupted %s internal parameter: found %s",
			luaNameSandboxContext, l.TypeOf(-1)))
	}
	return sandbox
}

// luaResetLog resets all properties of a log to their zero values.
// lua signature is function log_reset (logptr)
func luaResetLog(l *lua.State) int {
	log := getArgLogPtr(l, 1, luaNameResetFn)
	log.Reset()
	return 0
}

// luaRemoveLogProperty removes a property in the log with the given key. It returns true if it was removed;
// lua signature is function log_remove (logptr, key)
func luaRemoveLogProperty(l *lua.State) int {
	log := getArgLogPtr(l, 1, luaNameRemoveFn)
	key := getArgString(l, 2, luaNameRemoveFn)
	l.PushBoolean(log.Remove(key))
	return 1
}

// luaSetLogProperty sets a property in the log to the given value.
// It returns true of the property was upserted and false if property was created.
// lua signature is function log_set (logptr, key, value)
func luaSetLogProperty(l *lua.State) int {
	log := getArgLogPtr(l, 1, luaNameSetFn)
	key := getArgString(l, 2, luaNameSetFn)
	value := getArgString(l, 3, luaNameSetFn)
	l.PushBoolean(log.Set(key, value))
	return 1
}

// luaGetLogProperty returns the value of a property from the log or nil if log does not have property.
// It returns the property as a string or nil if log does not have property.
// lua signature is function log_get (logptr, key)
func luaGetLogProperty(l *lua.State) (i int) {
	log := getArgLogPtr(l, 1, luaNameGetFn)
	key := getArgString(l, 2, luaNameGetFn)

	var value string
	var ok bool
	value, ok = log.Get(key)
	if !ok {
		l.PushString("")
		return 1
	}
	l.PushString(value)
	return 1
}

func luaDebug(l *lua.State) int {
	// arg can be a string with a message, or a table with the fields to log
	switch l.TypeOf(1) {
	case lua.TypeString:
		s, _ := l.ToString(1)
		log.Info(s)
	case lua.TypeTable:
		fields := make(map[string]interface{})
		l.PushNil()

		for l.Next(1) {
			k, ok := l.ToString(-2)
			if !ok {
				panic(fmt.Errorf("table key must be a string in call to builtin '%s': found %s",
					luaNameDebugFn, l.TypeOf(-2)))
			}
			v := l.ToValue(-1)
			fields[k] = v
			l.Pop(1)
		}
		log.WithFields(log.Fields(fields)).Info()
	default:
		panic(fmt.Errorf(
			"%d argument must be a string or a table in call to builtin '%s' function: found %s",
			1, luaNameDebugFn, l.TypeOf(1)))
	}

	return 0
}

// luaSetConfig sets a property in the configuration to the given value.
// lua signature is function config_set (key, value)
func luaSetConfig(l *lua.State) int {
	key := getArgString(l, 1, luaNameConfigFn)
	sandbox := getStateSandbox(l)
	var err error

	switch key {
	case luaConfigProtected:
		err = sandbox.setProtected(getArgBool(l, 2, luaNameConfigFn+"#"+luaConfigProtected))
	case luaConfigTick:
		sandbox.setTick(getArgInt(l, 2, luaNameConfigFn+"#"+luaConfigTick))
	case luaConfigHTTPConcurrency:
		err = sandbox.setHTTPConcurrency(getArgInt(l, 2, luaNameConfigFn+"#"+luaConfigHTTPConcurrency))
	case luaConfigHTTPTimeout:
		err = sandbox.setHTTPTimeout(getArgString(l, 2, luaNameConfigFn+"#"+luaConfigHTTPTimeout))
	case luaConfigHTTPChannelBuffer:
		err = sandbox.setHTTPChannelBuffer(getArgInt(l, 2, luaNameConfigFn+"#"+luaConfigHTTPChannelBuffer))
	default:
		if !sandbox.setKafkaConfig(key, l.ToValue(2)) {
			err = fmt.Errorf("unknown config key in call to `%s`: '%s'. Available keys: %v",
				luaNameConfigFn, key, availableConfigKeys)
		}
	}
	if err != nil {
		lua.Errorf(l, "%s: %s", luaNameConfigFn, err)
		panic("unreachable")
	}
	return 0
}

// luaLogString will serialize the log and return it as a string in the otlog format.
// lua signature is function log_string(logptr) str
func luaLogString(l *lua.State) int {
	log := getArgLogPtr(l, 1, luaNameLogStringFn)
	l.PushString(log.String())
	return 1
}

// luaLogJSON will serialize the log and return it as a string in JSON format.
// lua signature is function log_JSON(logptr) str
func luaLogJSON(l *lua.State) int {
	log := getArgLogPtr(l, 1, luaNameLogStringFn)
	l.PushString(log.JSON())
	return 1
}

// used by runtime to provide better debugging when a lua runtime exception is thrown
func luaGoErrorHandler(l *lua.State) int {
	err, ok := l.ToString(-1)
	if !ok {
		panic(fmt.Errorf(
			"no error in call to system error handler: found %s", l.TypeOf(-1)))
	}
	lua.Traceback(l, l, err, 1)
	return 1
}
