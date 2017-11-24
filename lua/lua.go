package lua

import (
	"fmt"
	"os"
	"sync"
	"time"

	lua "github.com/Shopify/go-lua"
	"github.com/ernestrc/logd/http"
	"github.com/ernestrc/logd/logging"
)

const (
	/* internal */
	luaNameSandboxContext = "lsb_context"

	/* lua functions provided by client script */
	luaNameOnLogFn       = "on_log"
	luaNameOnTickFn      = "on_tick"
	luaNameOnHTTPErrorFn = "on_http_error"
)

type writers struct {
	stdout logging.LogWriter
	http   http.AsyncClient
}

// Sandbox represents a lua VM wich exposes a series of builtin functions
// to perform I/O operations and transformations over logging.Log structures.
type Sandbox struct {
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

func (l *Sandbox) setTick(tick int) {
	l.cfg.tick = tick
	// stop/start ticker if running
	if l.quitticker != nil {
		// in case setTick was called from ticker goroutine via on_tick lua function
		// global ticker log will prevent subsequent runTicker from starting
		// before the previous one has finished running
		go func(s *Sandbox) {
			s.quitticker <- struct{}{}
		}(l)
		if l.cfg.tick > 0 {
			go l.runTicker()
		}
	}
}

func (l *Sandbox) setHTTPChannelBuffer(c int) {
	cfg := l.write.http.Config()
	cfg.ChanBuffer = c
	l.write.http.Init(cfg, l.httpErrors)
}

func (l *Sandbox) setHTTPConcurrency(c int) {
	cfg := l.write.http.Config()
	cfg.Concurrency = c
	l.write.http.Init(cfg, l.httpErrors)
}

func (l *Sandbox) loadUtils() {
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

func (l *Sandbox) loadScript() error {
	if err := lua.DoFile(l.state, l.scriptPath); err != nil {
		return fmt.Errorf("there was an error when loading script: %s", err)
	}

	return nil
}

func (l *Sandbox) printStackInfo() {
	last, top := l.state.GetStackLastTop()
	fmt.Fprintf(os.Stderr, "last=%d top=%d\n", last, top)
}

func (l *Sandbox) callOnTick() bool {
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

func (l *Sandbox) runTicker() {
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

func (l *Sandbox) callOnHTTPError(e http.Error) {
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

func (l *Sandbox) pollHTTPErrors() {
	for err := range l.httpErrors {
		l.callOnHTTPError(err)
	}
}

// NewSandbox allocates storage and initializes a new Sandbox
func NewSandbox(scriptPath string, cfg *LuaConfig) (l *Sandbox, err error) {
	l = new(Sandbox)
	err = l.Init(scriptPath, cfg)
	return
}

// CallOnLog will call this lua sandbox's on_log function with the given log pointer
// or it will return an error if on_log is not defined.
func (l *Sandbox) CallOnLog(lg *logging.Log) error {
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
func (l *Sandbox) Init(scriptPath string, cfg *LuaConfig) (err error) {
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
	l.write.stdout = *logging.NewLogWriter(os.Stdout)
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
func (l *Sandbox) Close() {
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
