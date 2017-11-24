package lua

import (
	"fmt"
	"io/ioutil"
	stdHttp "net/http"

	lua "github.com/Shopify/go-lua"
	"github.com/ernestrc/logd/logging"
)

// TODO add kafka_post function
// TODO add elastic_post function
// TODO add graphite_post function
// TODO add netdata_post function
// TODO add s3_post function

const (
	luaNameConfigFn    = "config_set"
	luaNameHTTPGetFn   = "http_get"
	luaNameHTTPPostFn  = "http_post"
	luaNameGetFn       = "log_get"
	luaNameSetFn       = "log_set"
	luaNameRemoveFn    = "log_remove"
	luaNameResetFn     = "log_reset"
	luaNameLogStringFn = "log_string"
	luaNameLogJSONFn   = "log_json"
)

func getArgLogPtr(l *lua.State, i int, fn string) *logging.Log {
	log, ok := l.ToUserData(i).(*logging.Log)
	if !ok {
		panic(fmt.Sprintf(
			"%d argument must be a pointer to a Log structure in call to builtin '%s' function: found %s",
			i, fn, l.TypeOf(i)))
	}
	return log
}

func getArgString(l *lua.State, i int, fn string) string {
	arg, ok := l.ToString(i)
	if !ok {
		panic(fmt.Sprintf(
			"%d argument must be a string in call to builtin '%s' function: found %s",
			i, fn, l.TypeOf(i)))
	}
	return arg
}

func getArgInt(l *lua.State, i int, fn string) int {
	arg, ok := l.ToInteger(i)
	if !ok {
		panic(fmt.Sprintf(
			"%d argument must be an integer in call to builtin '%s' function: found %s",
			i, fn, l.TypeOf(i)))
	}
	return arg
}

func getStateSandbox(l *lua.State, i int) *Sandbox {
	l.Global(luaNameSandboxContext)
	sandbox, ok := l.ToUserData(i).(*Sandbox)
	if !ok {
		panic(fmt.Sprintf("corrupted %s internal parameter: found %s",
			luaNameSandboxContext, l.TypeOf(i)))
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
		l.PushNil()
		return 1
	}
	l.PushString(value)
	return 1
}

// luaSetConfig sets a property in the configuration to the given value.
// lua signature is function config_set (key, value)
func luaSetConfig(l *lua.State) int {
	key := getArgString(l, 1, luaNameConfigFn)
	sandbox := getStateSandbox(l, 3)

	switch key {
	case luaConfigTick:
		sandbox.setTick(getArgInt(l, 2, luaNameConfigFn+"#"+luaConfigTick))
	case luaConfigHTTPConcurrency:
		sandbox.setHTTPConcurrency(getArgInt(l, 2, luaNameConfigFn+"#"+luaConfigHTTPConcurrency))
	case luaConfigHTTPChannelBuffer:
		sandbox.setHTTPChannelBuffer(getArgInt(l, 2, luaNameConfigFn+"#"+luaConfigHTTPChannelBuffer))
	default:
		panic(fmt.Sprintf("unknown config key in call to `%s`: '%s'. Available keys: %v",
			luaNameConfigFn, key, availableConfigKeys))
	}
	return 0
}

// luaHTTPGet will make an HTTP request to the given url synchronously and return
// the body of the response and an error if there is one.
// lua signature is function http_get(url) body, err
func luaHTTPGet(l *lua.State) int {
	var b []byte
	url := getArgString(l, 1, luaNameHTTPGetFn)
	res, err := stdHttp.Get(url)

	if err != nil {
		goto errh
	}

	if b, err = ioutil.ReadAll(res.Body); err != nil {
		goto errh
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		err = fmt.Errorf("request to '%s' status: %s", url, res.Status)
		goto errh
	}

	l.PushString(string(b))
	l.PushNil()
	return 2

errh:
	l.PushNil()
	l.PushString(fmt.Sprintf("%s", err))
	return 2
}

// luaHTTPPost will POST the log to the given HTTP endpoint asynchronously.
// lua signature is function http_post(url, payload, contentType)
// Note that Content-Type is determined by the selected the output format via configuration.
func luaHTTPPost(l *lua.State) int {
	url := getArgString(l, 1, luaNameHTTPPostFn)
	payload := getArgString(l, 2, luaNameHTTPPostFn)
	contentType := getArgString(l, 3, luaNameHTTPPostFn)
	sandbox := getStateSandbox(l, 4)

	if sandbox.http == nil {
		sandbox.initHTTP()
	}

	// Avoid resource contention.
	// If http errors goroutine is trying to acquire this lock to call on_http_error lua fn
	// and the http requests channel is full, Post will block and thus create a deadlock
	sandbox.luaLock.Unlock()
	defer sandbox.luaLock.Lock()
	_, err := sandbox.http.Post(url, payload, contentType)
	if err != nil {
		panic(err)
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
