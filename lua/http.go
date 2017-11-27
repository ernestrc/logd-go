package lua

import (
	"fmt"
	"io/ioutil"
	stdHttp "net/http"

	lua "github.com/Shopify/go-lua"
	"github.com/ernestrc/logd/http"
)

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
