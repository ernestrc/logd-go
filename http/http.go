package http

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

// AsyncClient is an HTTP client which submits requests asynchronously and concurrently.
// Content of the responses is ignored and errors are reported via an non-buffered channel.
// This client is useful for fire and forget use cases.
type AsyncClient struct {
	cfg       Config
	reqchan   chan *http.Request
	errorchan chan<- Error
	quitchan  chan struct{}
}

// Config is a AsyncClient configuration
type Config struct {
	Concurrency int
	ChanBuffer  int
}

// DefaultConfig is a client config with sane defaults
var DefaultConfig = Config{Concurrency: 4, ChanBuffer: 100}

// Error is an HTTP request error. This includes both connectivity errors and non-2XX responses.
type Error struct {
	Request *http.Request
	Err     error
}

func postRequest(client *http.Client, req *http.Request) error {
	res, err := client.Do(req)
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

func poster(id int, reqchan chan *http.Request, errorchan chan<- Error, quitchan chan struct{}) {
	var client http.Client
	for req := range reqchan {
		if err := postRequest(&client, req); err != nil {
			if errorchan != nil {
				errorchan <- Error{req, err}
			}
		}
	}
	quitchan <- struct{}{}
}

// NewClient allocates enough space to store an AsyncClient and initializes it.
// If configuration is nil a default one will be used.
func NewClient(errorchan chan<- Error) *AsyncClient {
	w := new(AsyncClient)
	w.Init(nil, errorchan)
	return w
}

// Init initializes this AsyncClient so it is ready for use.
// Calling Init after it is initialized will call Close first, flushing all the pending data, and re-initialize it.
// After calling this method, changes to cfg will not have any effect. If new configuration is to be used, call Init again with
// the updated configuration and client will updated.
func (w *AsyncClient) Init(cfg *Config, errorchan chan<- Error) {
	if w.reqchan != nil {
		w.Close()
	}
	if cfg == nil {
		w.cfg = DefaultConfig
	} else {
		w.cfg = *cfg
	}
	w.errorchan = errorchan
	w.reqchan = make(chan *http.Request, w.cfg.ChanBuffer)
	w.quitchan = make(chan struct{})
	for i := 0; i < w.cfg.Concurrency; i++ {
		go poster(i, w.reqchan, w.errorchan, w.quitchan)
	}
}

// Post makes an HTTP post to the given url and sets the Content-Type header accordingly
func (w *AsyncClient) Post(url string, payload string, contentType string) (n int, err error) {
	var req *http.Request

	if req, err = http.NewRequest("POST", url, strings.NewReader(payload)); err != nil {
		return
	}
	req.Header.Add("Content-Type", contentType)

	if w.reqchan == nil {
		panic("called Write before writer was initialized or after Flush was called")
	}

	w.reqchan <- req
	return
}

// Close will block until all data has been written.
// In order to use again this client instance Init must be used to initialize its resources
func (w *AsyncClient) Close() error {
	close(w.reqchan)
	// wait for goroutines to finish the work
	for i := 0; i < w.cfg.Concurrency; i++ {
		<-w.quitchan
	}
	close(w.quitchan)
	// marks client as uninitialized
	w.reqchan = nil
	return nil
}
