package lua

import (
	"fmt"
	"os"
	"strings"
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

	/* lua functions provided by client script */
	luaNameOnLogFn        = "on_log"
	luaNameOnTickFn       = "on_tick"
	luaNameOnHTTPErrorFn  = "on_http_error"
	luaNameOnKafkaErrorFn = "on_kafka_error"
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

func (l *Sandbox) setKafkaConfig(key string, value interface{}) bool {
	if !strings.HasPrefix(key, "kafka.") {
		return false
	}
	key = strings.TrimLeft(key, "kafka.")
	l.kafkaConfig.SetKey(key, value)
	// FIXME tear down and re-initialize
	// if l.kafka != nil {
	// 	l.kafka.Close()
	// }
	return true
}

func (l *Sandbox) setHTTPChannelBuffer(c int) {
	l.httpConfig.ChanBuffer = c
	if l.http != nil {
		l.http.Init(l.httpConfig, l.httpErrors)
	}
}

func (l *Sandbox) setHTTPConcurrency(c int) {
	l.httpConfig.Concurrency = c
	if l.http != nil {
		l.http.Init(l.httpConfig, l.httpErrors)
	}
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

	l.state.PushGoFunction(luaKafkaOffset)
	l.state.SetGlobal(luaNameKafkaOffsetFn)

	l.state.PushGoFunction(luaKafkaPost)
	l.state.SetGlobal(luaNameKafkaPostFn)

	l.state.PushUserData(l)
	l.state.SetGlobal(luaNameSandboxContext)
}

func (l *Sandbox) loadScript() error {
	if err := lua.DoFile(l.state, l.scriptPath); err != nil {
		panic(err)
		// return fmt.Errorf("there was an error when loading script: %s", err)
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
	l.state.Call(4, 0)
}

func (l *Sandbox) callOnKafkaError(m *kafka.Message) {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()
	l.state.Global(luaNameOnKafkaErrorFn)
	if !l.state.IsFunction(-1) {
		l.state.Pop(-1)
		return
	}

	topic := *m.TopicPartition.Topic
	partition := int(m.TopicPartition.Partition)
	offset := int(m.TopicPartition.Offset)
	err := m.TopicPartition.Error.Error()

	l.state.PushString(topic)
	l.state.PushInteger(partition)
	l.state.PushInteger(offset)
	l.state.PushString(fmt.Sprintf("error when producing message to topic '%s' at partition %d with offset %d: %s",
		topic, partition, offset, err))
	l.state.PushString(string(m.Value))
	l.state.Call(5, 0)
}

func (l *Sandbox) pollHTTPErrors() {
	for err := range l.httpErrors {
		l.callOnHTTPError(err)
	}
}

func (l *Sandbox) pollKafkaEvents() {
	for ev := range l.kafka.Events() {
		switch ev.(type) {
		case *kafka.Message:
			m := ev.(*kafka.Message)
			l.callOnKafkaError(m)
		case *kafka.Error:
			// TODO do something with error
		default:
			panic(fmt.Sprintf("unexpected kafka event: %s", ev))
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
	l.state.Global(luaNameOnLogFn)
	if !l.state.IsFunction(-1) {
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

	lua.OpenLibraries(l.state)
	l.loadUtils()
	if err = l.initKafka(); err != nil {
		return
	}
	if err = l.loadScript(); err != nil {
		return
	}
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
	if l.quitticker != nil {
		l.quitticker <- struct{}{}
		close(l.quitticker)
		l.quitticker = nil
	}
	if l.http != nil {
		l.http.Close()
		close(l.httpErrors)
	}
	if l.kafka != nil {
		// FIXME
		l.kafka.Close()
	}
	// marks sandbox as uninitialized
	l.state = nil
}
