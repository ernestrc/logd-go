package lua

import (
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	lua "github.com/Shopify/go-lua"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/ernestrc/logd/http"
	"github.com/ernestrc/logd/logging"
)

const (
	/* internal */
	luaNameSandboxContext = "lsb_context"
	luaNameLogdModule     = "logd"

	/* lua functions provided by client script */
	luaNameOnLogFn         = "on_log"
	luaNameOnErrorFn       = "on_error"
	luaNameOnTickFn        = "on_tick"
	luaNameOnHTTPErrorFn   = "on_http_error"
	luaNameOnKafkaReportFn = "on_kafka_report"
)

// Sandbox represents a lua VM wich exposes a series of builtin functions
// to perform I/O operations and transformations over logging.Log structures.
type Sandbox struct {
	luaLock     sync.Mutex
	scriptPath  string
	cfg         sandboxConfig
	state       *lua.State
	httpConfig  *http.Config
	http        *http.AsyncClient
	kafkaConfig *kafka.ConfigMap
	kafka       *kafka.Producer
	quitticker  chan struct{}
	httpErrors  chan http.Error
}

func (l *Sandbox) stopTicker() {
	if l.quitticker == nil {
		return
	}
	l.quitticker <- struct{}{}
	close(l.quitticker)
	l.quitticker = nil // indicating that ticker is not running
}

func (l *Sandbox) restartTicker() {
	l.stopTicker()
	l.runTicker()
}

func (l *Sandbox) setTick(tick int) {
	l.cfg.tick = tick
	// stop/start ticker if running. In case setTick was called
	// from on_tick hook, run stop/start in a separate goroutine
	// so we don't deadlock
	go l.restartTicker()
}

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
	/* hooks are left undefined
	{Name: luaNameOnLogFn, Function: nil},
	{Name: luaNameOnTickFn, Function: nil},
	{Name: luaNameOnHTTPErrorFn, Function: nil},
	{Name: luaNameOnKafkaReportFn, Function: nil},
	*/
}

// opens the logd library
func luaLogdOpen(l *lua.State) int {
	lua.NewLibrary(l, logdAPI)
	return 1
}

func (l *Sandbox) addPackagePath(path string) {
	l.state.Global("package")
	l.state.Field(-1, "path")
	l.state.PushString(fmt.Sprintf(";%s", path))
	l.state.Concat(2)
	l.state.SetField(-2, "path")
	l.state.Pop(1)
}

func (l *Sandbox) openLogdLibrary() {
	lua.Require(l.state, luaNameLogdModule, luaLogdOpen, true)
	l.state.Pop(1)

	lua.SubTable(l.state, lua.RegistryIndex, "_PRELOAD")
	l.state.PushGoFunction(luaLogdOpen)
	l.state.SetField(-2, luaNameLogdModule)
	l.state.Pop(1)

	l.state.PushUserData(l)
	l.state.SetGlobal(luaNameSandboxContext)
}

func (l *Sandbox) loadUserScript() error {
	if err := lua.DoFile(l.state, l.scriptPath); err != nil {
		return fmt.Errorf("there was an error when loading script: %s", err)
	}

	return nil
}

func (l *Sandbox) tick() {
	ticker := time.NewTicker(time.Duration(l.cfg.tick * 1000 * 1000))
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			var fn func() error
			if l.cfg.protected {
				fn = l.callProtectedOnTick
			} else {
				fn = l.callOnTick
			}
			if err := fn(); err != nil {
				panic(err)
			}
		case <-l.quitticker:
			return
		}
	}
}

func (l *Sandbox) runTicker() {
	l.quitticker = make(chan struct{})
	go l.tick()
}

// NewSandbox allocates storage and initializes a new Sandbox
func NewSandbox(scriptPath string) (l *Sandbox, err error) {
	l = new(Sandbox)
	if err = l.Init(scriptPath); err != nil {
		l = nil
	}
	return
}

// caller must take care of synchronizing concurrent access to state
// and it's responsible for popping the logd module from the stack
func (l *Sandbox) pushOnTick() (err error) {
	l.state.Global(luaNameLogdModule)
	l.state.Field(-1, luaNameOnTickFn)
	if ok := l.state.IsFunction(-1); !ok {
		l.state.Pop(1)
		err = fmt.Errorf("not defined in lua script: function logd.on_tick()")
		return
	}
	return
}

// caller must take care of synchronizing concurrent access to state
// and it's responsible for popping the logd module from the stack
func (l *Sandbox) pushOnLog(lg *logging.Log) (err error) {
	l.state.Global(luaNameLogdModule)
	l.state.Field(-1, luaNameOnLogFn)
	if !l.state.IsFunction(-1) {
		l.state.Pop(1)
		err = fmt.Errorf("not defined in lua script: function logd.on_log (logptr)")
		return
	}
	l.state.PushUserData(lg)

	return
}

// note that caller is responsible for pushing the system error handler in the stack at index 1
func (l *Sandbox) callProtected(lg *logging.Log, args, ret int, fnName string) error {
	const errHandlerIdx = 1
	if !l.state.IsFunction(errHandlerIdx) {
		panic(fmt.Errorf("could not find system error handler at index %d", errHandlerIdx))
	}

	// if error is handled by hook, we do not need to return it
	if runtimeErr := l.state.ProtectedCall(args, ret, errHandlerIdx); runtimeErr != nil {
		if _, ok := runtimeErr.(lua.RuntimeError); !ok {
			return runtimeErr
		}
		l.callOnError(lg, fmt.Errorf("%s : %s", fnName, runtimeErr))
	}

	return nil
}

func (l *Sandbox) callOnTick() (err error) {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()

	err = l.pushOnTick()
	defer l.state.Pop(1)
	if err != nil {
		return
	}

	l.state.Call(0, 0)
	return
}

func (l *Sandbox) callProtectedOnTick() (err error) {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()

	l.state.PushGoFunction(luaGoErrorHandler)
	err = l.pushOnTick()
	defer l.state.Pop(1)
	if err != nil {
		return
	}

	err = l.callProtected(&logging.Log{}, 0, 0, luaNameOnTickFn)
	return
}

func (l *Sandbox) callOnError(lg *logging.Log, err error) {
	l.state.Global(luaNameLogdModule)
	defer l.state.Pop(1)

	l.state.Field(-1, luaNameOnErrorFn)
	if !l.state.IsFunction(-1) {
		l.state.Pop(1)
		panic(fmt.Errorf("runtime error thrown but not defined in lua script: function %s.%s(logptr, error): %s",
			luaNameLogdModule, luaNameOnErrorFn, err))
	}
	l.state.PushUserData(lg)
	l.state.PushString(err.Error())
	l.state.Call(2, 0)
}

// CallOnLog will call this lua sandbox's on_log function with the given log pointer
// or it will return an error if on_log is not defined.
func (l *Sandbox) CallOnLog(lg *logging.Log) (err error) {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()

	err = l.pushOnLog(lg)
	defer l.state.Pop(1)
	if err != nil {
		return
	}

	l.state.Call(1, 0)

	return
}

// ProtectedCallOnLog behaves like CallOnLog except that if there's a lua
// runtime error, it will look for 'on_error' hook and call it.
func (l *Sandbox) ProtectedCallOnLog(lg *logging.Log) (err error) {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()

	l.state.PushGoFunction(luaGoErrorHandler)
	err = l.pushOnLog(lg)
	defer l.state.Pop(1)
	if err != nil {
		return
	}

	err = l.callProtected(lg, 1, 0, luaNameOnLogFn)

	return
}

func (l *Sandbox) errorHandlerDefined() bool {
	l.state.Global(luaNameLogdModule)
	l.state.Field(-1, luaNameOnErrorFn)
	defer l.state.Pop(2)
	return l.state.IsFunction(-1)
}

func (l *Sandbox) initHTTP() (err error) {
	l.httpErrors = make(chan http.Error)
	if l.http, err = http.NewClient(l.httpConfig, l.httpErrors); err != nil {
		return
	}
	go l.pollHTTPErrors()

	return
}

func (l *Sandbox) initKafka() (err error) {
	l.kafka, err = kafka.NewProducer(l.kafkaConfig)
	if err != nil {
		return
	}
	go l.pollKafkaEvents()
	return
}

func (l *Sandbox) setProtected(enabled bool) (err error) {
	if enabled && !l.errorHandlerDefined() {
		err = fmt.Errorf("protected mode set but not defined: function %s.%s (logptr, error)",
			luaNameLogdModule, luaNameOnErrorFn)
		return
	}
	l.cfg.protected = enabled
	return
}

// Init initializes l by instantiating a fresh lua state and loading the given script
// along with the standard lua libraries in it. If cfg is nil, a default configuration is used.
func (l *Sandbox) Init(scriptPath string) (err error) {
	if l.state != nil {
		l.Close()
	}
	l.luaLock.Lock()
	defer l.luaLock.Unlock()
	l.state = lua.NewState()
	l.scriptPath = scriptPath

	httpConfig := http.DefaultConfig
	l.httpConfig = &httpConfig

	kafkaConfig := kafka.ConfigMap(make(map[string]kafka.ConfigValue))
	l.kafkaConfig = &kafkaConfig
	setSaneKafkaDefaults(l.kafkaConfig)

	lua.OpenLibraries(l.state)
	l.openLogdLibrary()

	/* enable script to require modules that are relative path-wise */
	scriptDir := path.Dir(scriptPath)
	if !path.IsAbs(scriptPath) {
		var cwd string
		if cwd, err = os.Getwd(); err != nil {
			return
		}
		scriptDir = path.Join(cwd, scriptDir)
	}
	l.addPackagePath(path.Join(scriptDir, "?.lua"))

	if err = l.loadUserScript(); err != nil {
		return
	}

	if l.cfg.protected {
		if err = l.setProtected(l.cfg.protected); err != nil {
			return
		}
	}

	l.restartTicker()
	return
}

// ProtectedMode returns whether sandbox is configured to run in protected mode.
// In this mode, when a lua runtime error is thrown by function logd.on_log,
// function logd.on_error(logptr, error) is called.
// This mode is toggled by calling logd.config_set("protected", true/false)
// or by the initial sandbox configuration
func (l *Sandbox) ProtectedMode() bool {
	return l.cfg.protected
}

// Flush will try to flush all pending I/O operations.
func (l *Sandbox) Flush() {
	if l.kafka != nil {
		l.flushKafka()
	}
	if l.http != nil {
		l.http.Flush()
	}
}

// Close will shut down all the resources held by this Sandbox and flush all the
// pending I/O operations. Init must be called again if this instance is to be used.
func (l *Sandbox) Close() {
	if l.kafka != nil {
		l.flushKafka()
		l.kafka.Close()
		l.kafka = nil
	}
	l.luaLock.Lock()
	defer l.luaLock.Unlock()

	l.stopTicker()

	if l.http != nil {
		l.http.Close()
		close(l.httpErrors)
		l.httpErrors = nil
		l.http = nil
	}

	// marks sandbox as uninitialized
	l.state = nil
}
