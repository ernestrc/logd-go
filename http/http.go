package http

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	uuid "github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// AsyncClient is an HTTP client which submits requests asynchronously and concurrently.
// Content of the responses is ignored and errors are reported via an non-buffered channel.
// This client is useful for fire and forget use cases.
type AsyncClient struct {
	cfg Config
	// used to synchronize flush and new request submits
	lock      sync.Mutex
	reqchan   []chan *request
	errorchan chan<- Error
	quitchan  chan struct{}
}

// Config is a AsyncClient configuration
type Config struct {
	Concurrency int
	ChanBuffer  int
}

type request struct {
	*http.Request
	time.Time
	ctx *log.Entry
}

// DefaultConfig is a client config with sane defaults
var DefaultConfig = Config{Concurrency: 4, ChanBuffer: 100}

func validateConfiguration(cfg *Config) (err error) {
	if cfg == nil {
		panic(fmt.Errorf("cannot validate nil configuration"))
	}
	if cfg.Concurrency < 1 {
		err = fmt.Errorf("config error: min http concurrency is 1")
		return
	}
	if cfg.ChanBuffer < 1 {
		err = fmt.Errorf("config error: min http channel buffer is 1")
		return
	}

	return
}

// Error is an HTTP request error. This includes both connectivity errors and non-2XX responses.
type Error struct {
	Request *request
	Err     error
}

func postRequest(client *http.Client, req *request) error {
	res, err := client.Do(req.Request)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	if b, err := ioutil.ReadAll(res.Body); err != nil {
		return fmt.Errorf("request to '%s' status: %+v: %s", req.URL.String(), res.Status, string(b))
	}
	return fmt.Errorf("request to '%s' status: %+v", req.URL.String(), res.Status)
}

func poster(id int, reqchan chan *request, errorchan chan<- Error, quitchan chan struct{}) {
	var client http.Client
	for req := range reqchan {
		req.ctx.WithFields(log.Fields{
			"tag":    "HttpPostAttempt",
			"worker": id,
		}).Debug()
		err := postRequest(&client, req)
		duration := time.Now().UnixNano() - req.Time.UnixNano()
		if err != nil {
			if errorchan != nil {
				errorchan <- Error{req, err}
			}
			req.ctx.WithFields(log.Fields{
				"tag":      "HttpPostFailure",
				"error":    err,
				"duration": duration,
			}).Debug()
		} else {
			req.ctx.WithFields(log.Fields{
				"tag":      "HttpPostSuccess",
				"duration": duration,
			}).Debug()
		}
	}
	quitchan <- struct{}{}
}

// NewClient allocates enough space to store an AsyncClient and initializes it.
// If configuration is nil a default one will be used.
func NewClient(errorchan chan<- Error) (a *AsyncClient, err error) {
	a = new(AsyncClient)
	if err = a.Init(nil, errorchan); err != nil {
		a = nil
		return
	}
	return
}

// Concurrency returns the configured max number of concurrent requests
func (a *AsyncClient) Concurrency() int {
	return a.cfg.Concurrency
}

func (a *AsyncClient) initWorkers() {
	a.reqchan = make([]chan *request, a.cfg.Concurrency)
	for i := 0; i < a.cfg.Concurrency; i++ {
		a.reqchan[i] = make(chan *request, a.cfg.ChanBuffer)
		go poster(i, a.reqchan[i], a.errorchan, a.quitchan)
	}
}

// Init initializes this AsyncClient so it is ready for use.
// Calling Init after it is initialized will call Close first, flushing all the pending data, and re-initialize it.
// After calling this method, changes to cfg will not have any effect. If new configuration is to be used, call Init again with
// the updated configuration and client will updated.
func (a *AsyncClient) Init(cfg *Config, errorchan chan<- Error) (err error) {
	if a.reqchan != nil {
		if err = a.Close(); err != nil {
			return
		}
	}
	if cfg == nil {
		a.cfg = DefaultConfig
	} else {
		a.cfg = *cfg
	}
	if err = validateConfiguration(&a.cfg); err != nil {
		return
	}
	a.errorchan = errorchan
	a.quitchan = make(chan struct{})
	a.initWorkers()

	return
}

func newPostRequest(ctx *log.Entry, url, contentType, payload string) (*request, error) {
	time := time.Now()
	req, err := http.NewRequest("POST", url, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", contentType)

	return &request{
		req,
		time,
		ctx,
	}, nil
}

// Post makes an HTTP post to the given url and sets the Content-Type header accordingly
func (a *AsyncClient) Post(url string, payload string, contentType string, affinity int) (err error) {
	maxAffinity := a.cfg.Concurrency - 1
	if affinity > maxAffinity {
		err = fmt.Errorf("Post: cannot pass affinity greater than %d (only %d channels available)", maxAffinity, a.cfg.Concurrency)
		return
	}

	traceID := uuid.New().String()
	ctx := log.WithFields(log.Fields{
		"tag":      "HttpPostSubmit",
		"url":      url,
		"class":    "AsyncHTTPClient",
		"affinity": affinity,
		"traceId":  traceID,
	})

	ctx.Debug()

	var req *request
	if req, err = newPostRequest(ctx, url, contentType, payload); err != nil {
		return
	}

	if a.reqchan == nil {
		panic(fmt.Errorf("called Write before writer was initialized or after Close was called"))
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	if affinity >= 0 {
		a.reqchan[affinity] <- req
		return
	}

	// find an available channel
	for i := 0; i < a.cfg.Concurrency; i++ {
		select {
		case a.reqchan[i] <- req:
			return
		default:
		}
	}

	// if not just block caller
	a.reqchan[0] <- req
	return
}

// Flush all pending I/O operations
func (a *AsyncClient) Flush() {
	a.lock.Lock()
	defer a.lock.Unlock()
	a.stopWorkers()
	a.initWorkers()
}

func (a *AsyncClient) stopWorkers() {
	for i := 0; i < a.cfg.Concurrency; i++ {
		close(a.reqchan[i])
		<-a.quitchan
	}
}

// Close will block until all data has been written.
// In order to use again this client instance Init must be used to initialize its resources
func (a *AsyncClient) Close() error {
	// wait for goroutines to finish the work
	a.stopWorkers()
	close(a.quitchan)
	// marks client as uninitialized
	a.reqchan = nil
	return nil
}
