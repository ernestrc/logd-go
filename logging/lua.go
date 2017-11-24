package logging

import (
	"fmt"
	"io/ioutil"
	stdHttp "net/http"
	"os"
	"sync"
	"time"

	lua "github.com/Shopify/go-lua"
	"github.com/ernestrc/logd/http"
)

const getLineQuery = "nSl"

const (
	/* internal */
	luaNameSandboxContext = "lsb_context"

	/* builtin lua functions provided */
	luaNameConfigFn    = "config_set"
	luaNameHTTPGetFn   = "http_get"
	luaNameHTTPPostFn  = "http_post"
	luaNameGetFn       = "log_get"
	luaNameSetFn       = "log_set"
	luaNameRemoveFn    = "log_remove"
	luaNameResetFn     = "log_reset"
	luaNameLogStringFn = "log_string"
	luaNameLogJSONFn   = "log_json"
	// TODO add kafka_post function
	// TODO add elastic_post function
	// TODO add graphite_post function
	// TODO add netdata_post function
	// TODO add s3_post function

	/* lua functions provided by client script */
	luaNameOnLogFn       = "on_log"
	luaNameOnTickFn      = "on_tick"
	luaNameOnHTTPErrorFn = "on_http_error"
)

/* configuration updated via builtin `config(key str, value str)`*/
const (
	luaConfigTick              = "tick"
	luaConfigHTTPConcurrency   = "http_concurrency"
	luaConfigHTTPChannelBuffer = "http_channel_buffer"
)

var availableConfigKeys = []string{
	luaConfigTick,
	luaConfigHTTPConcurrency,
	luaConfigHTTPChannelBuffer,
}

type writers struct {
	stdout LogWriter
	http   http.AsyncClient
}

// LuaConfig represents the configuration of a LuaSandbox
type LuaConfig struct {
	// general
	tick int
}

// LuaSandbox represents a lua VM wich exposes a series of builtin functions
// to perform I/O operations and transformations over Log structures.
type LuaSandbox struct {
	luaLock    sync.Mutex
	tickerLock sync.Mutex
	state      *lua.State
	write      writers
	cfg        *LuaConfig
	scriptPath string
	// channel to internally terminate ticker
	quitticker chan struct{}
	httpErrors chan http.Error
}

func getArgLogPtr(l *lua.State, i int, fn string) *Log {
	log, ok := l.ToUserData(i).(*Log)
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

func getStateSandbox(l *lua.State, i int) *LuaSandbox {
	l.Global(luaNameSandboxContext)
	sandbox, ok := l.ToUserData(i).(*LuaSandbox)
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

func (l *LuaSandbox) setTick(tick int) {
	l.cfg.tick = tick
	// stop/start ticker if running
	if l.quitticker != nil {
		// in case setTick was called from ticker goroutine via on_tick lua function
		// global ticker log will prevent subsequent runTicker from starting
		// before the previous one has finished running
		go func(s *LuaSandbox) {
			s.quitticker <- struct{}{}
		}(l)
		if l.cfg.tick > 0 {
			go l.runTicker()
		}
	}
}

func (l *LuaSandbox) setHTTPChannelBuffer(c int) {
	cfg := l.write.http.Config()
	cfg.ChanBuffer = c
	l.write.http.Init(cfg, l.httpErrors)
}

func (l *LuaSandbox) setHTTPConcurrency(c int) {
	cfg := l.write.http.Config()
	cfg.Concurrency = c
	l.write.http.Init(cfg, l.httpErrors)
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

	// Avoid resource contention.
	// If http errors goroutine is trying to acquire this lock to call on_http_error lua fn
	// and the http requests channel is full, Post will block and thus create a deadlock
	sandbox.luaLock.Unlock()
	defer sandbox.luaLock.Lock()
	_, err := sandbox.write.http.Post(url, payload, contentType)
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

func (l *LuaSandbox) loadUtils() {
	l.state.PushGoFunction(luaHTTPGet)
	l.state.SetGlobal(luaNameHTTPGetFn)

	l.state.PushGoFunction(luaHTTPPost)
	l.state.SetGlobal(luaNameHTTPPostFn)

	l.state.PushGoFunction(luaSetConfig)
	l.state.SetGlobal(luaNameConfigFn)

	l.state.PushGoFunction(luaGetLogProperty)
	l.state.SetGlobal(luaNameGetFn)

	l.state.PushGoFunction(luaSetLogProperty)
	l.state.SetGlobal(luaNameSetFn)

	l.state.PushGoFunction(luaRemoveLogProperty)
	l.state.SetGlobal(luaNameRemoveFn)

	l.state.PushGoFunction(luaResetLog)
	l.state.SetGlobal(luaNameResetFn)

	l.state.PushGoFunction(luaLogString)
	l.state.SetGlobal(luaNameLogStringFn)

	l.state.PushGoFunction(luaLogJSON)
	l.state.SetGlobal(luaNameLogJSONFn)

	l.state.PushUserData(l)
	l.state.SetGlobal(luaNameSandboxContext)
}

func (l *LuaSandbox) loadScript() error {
	if err := lua.DoFile(l.state, l.scriptPath); err != nil {
		return fmt.Errorf("there was an error when loading script: %s", err)
	}

	return nil
}

func (l *LuaSandbox) printStackInfo() {
	last, top := l.state.GetStackLastTop()
	fmt.Fprintf(os.Stderr, "last=%d top=%d\n", last, top)
}

func (l *LuaSandbox) callOnTick() bool {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()
	l.state.Global(luaNameOnTickFn)
	if l.state.IsFunction(-1) {
		l.state.Call(0, 0)
		return true
	}
	l.state.Pop(-1)
	return false
}

func (l *LuaSandbox) runTicker() {
	l.tickerLock.Lock()
	defer l.tickerLock.Unlock()
	ticker := time.NewTicker(time.Duration(l.cfg.tick * 1000 * 1000))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			defined := l.callOnTick()
			// stop goroutine if on_tick is not defined
			if !defined {
				panic("ticker goroutine error: called ticker but `on_tick` is not defined")
			}
		case <-l.quitticker:
			return
		}
	}
}

func (l *LuaSandbox) callOnHTTPError(e http.Error) {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()
	l.state.Global(luaNameOnHTTPErrorFn)
	if !l.state.IsFunction(-1) {
		l.state.Pop(-1)
		return
	}

	url := e.Request.URL.String()
	method := e.Request.Method
	err := fmt.Sprintf("%s", e.Err)

	l.state.PushString(url)
	l.state.PushString(method)
	l.state.PushString(err)
	l.state.Call(3, 0)
}

func (l *LuaSandbox) pollHTTPErrors() {
	for err := range l.httpErrors {
		l.callOnHTTPError(err)
	}
}

// NewLuaSandbox allocates storage and initializes a new LuaSandbox
func NewLuaSandbox(scriptPath string, cfg *LuaConfig) (l *LuaSandbox, err error) {
	l = new(LuaSandbox)
	err = l.Init(scriptPath, cfg)
	return
}

// CallOnLog will call this lua sandbox's on_log function with the given log pointer
// or it will return an error if on_log is not defined.
func (l *LuaSandbox) CallOnLog(lg *Log) error {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()
	l.state.Global(luaNameOnLogFn)
	if !l.state.IsFunction(-1) {
		return fmt.Errorf("not defined in lua script: function on_log (logptr)")
	}
	l.state.PushUserData(lg)
	l.state.Call(1, 0)
	return nil
}

// Init initializes l by instantiating a fresh lua state and loading the given script
// along with the standard lua libraries in it. If cfg is nil, a default configuration is used.
func (l *LuaSandbox) Init(scriptPath string, cfg *LuaConfig) (err error) {
	if l.state != nil {
		l.Close()
	}
	if cfg != nil {
		l.cfg = cfg
	} else {
		l.cfg = &LuaConfig{
			tick: 0,
		}
	}
	l.httpErrors = make(chan http.Error)
	l.state = lua.NewState()
	l.scriptPath = scriptPath
	l.write.stdout = *NewLogWriter(os.Stdout)
	l.write.http.Init(http.DefaultClientConfig, l.httpErrors)

	lua.OpenLibraries(l.state)
	l.loadUtils()
	if err = l.loadScript(); err != nil {
		return
	}
	go l.pollHTTPErrors()
	if l.cfg.tick > 0 {
		l.quitticker = make(chan struct{})
		go l.runTicker()
	}
	return nil
}

// Close will shut down all the resources held by this Sandbox and flush all the
// pending I/O operations. Init must be called again if this instance is to be used.
func (l *LuaSandbox) Close() {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()
	l.write.stdout.Flush()
	l.write.http.Close()
	if l.quitticker != nil {
		l.quitticker <- struct{}{}
		close(l.quitticker)
		l.quitticker = nil
	}
	// exit http errors goroutine
	close(l.httpErrors)
	// marks sandbox as uninitialized
	l.state = nil
}
