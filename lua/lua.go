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
	luaNameOnTickFn        = "on_tick"
	luaNameOnHTTPErrorFn   = "on_http_error"
	luaNameOnKafkaReportFn = "on_kafka_report"
)

// Sandbox represents a lua VM wich exposes a series of builtin functions
// to perform I/O operations and transformations over logging.Log structures.
type Sandbox struct {
	luaLock     sync.Mutex
	tickerLock  sync.Mutex
	scriptPath  string
	cfg         *Config
	state       *lua.State
	httpConfig  *http.Config
	http        *http.AsyncClient
	kafkaConfig *kafka.ConfigMap
	kafka       *kafka.Producer
	quitticker  chan struct{}
	httpErrors  chan http.Error
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

func (l *Sandbox) callOnTick() (ok bool) {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()

	l.state.Global(luaNameLogdModule)
	defer l.state.Pop(1)

	l.state.Field(-1, luaNameOnTickFn)
	ok = l.state.IsFunction(-1)
	if !ok {
		l.state.Pop(1)
		return
	}
	l.state.Call(0, 0)
	return
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

// NewSandbox allocates storage and initializes a new Sandbox
func NewSandbox(scriptPath string, cfg *Config) (l *Sandbox, err error) {
	l = new(Sandbox)
	err = l.Init(scriptPath, cfg)
	return
}

// CallOnLog will call this lua sandbox's on_log function with the given log pointer
// or it will return an error if on_log is not defined.
func (l *Sandbox) CallOnLog(lg *logging.Log) error {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()

	l.state.Global(luaNameLogdModule)
	defer l.state.Pop(1)

	l.state.Field(-1, luaNameOnLogFn)
	if !l.state.IsFunction(-1) {
		l.state.Pop(1)
		return fmt.Errorf("not defined in lua script: function on_log (logptr)")
	}
	l.state.PushUserData(lg)
	l.state.Call(1, 0)
	return nil
}

func (l *Sandbox) initHTTP() {
	l.httpErrors = make(chan http.Error)
	l.http = http.NewClient(l.httpErrors)
	go l.pollHTTPErrors()
}

func (l *Sandbox) initKafka() (err error) {
	l.kafka, err = kafka.NewProducer(l.kafkaConfig)
	if err != nil {
		return
	}
	go l.pollKafkaEvents()
	return
}

// Init initializes l by instantiating a fresh lua state and loading the given script
// along with the standard lua libraries in it. If cfg is nil, a default configuration is used.
func (l *Sandbox) Init(scriptPath string, cfg *Config) (err error) {
	if l.state != nil {
		l.Close()
	}
	if cfg != nil {
		l.cfg = cfg
	} else {
		l.cfg = &Config{
			tick: 0,
		}
	}
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
	var cwd string
	if cwd, err = os.Getwd(); err != nil {
		return
	}
	scriptDir := path.Join(cwd, path.Dir(scriptPath))
	l.addPackagePath(path.Join(scriptDir, "?.lua"))

	if err = l.loadUserScript(); err != nil {
		return
	}

	if l.cfg.tick > 0 {
		l.quitticker = make(chan struct{})
		go l.runTicker()
	}
	return nil
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
	}
	l.luaLock.Lock()
	defer l.luaLock.Unlock()
	if l.quitticker != nil {
		l.quitticker <- struct{}{}
		close(l.quitticker)
		l.quitticker = nil
	}
	if l.http != nil {
		l.http.Close()
		close(l.httpErrors)
	}

	// marks sandbox as uninitialized
	l.state = nil
}
