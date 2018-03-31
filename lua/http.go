package lua

import (
	"fmt"
	"io/ioutil"
	stdHttp "net/http"
	"time"

	lua "github.com/Shopify/go-lua"
	"github.com/ernestrc/logd/http"
)

// luaHTTPGet will make an HTTP request to the given url synchronously and return
// the body of the response and an error if there is one.
// lua signature is function http_get(url, [headers]) body, err
func luaHTTPGet(l *lua.State) int {
	var b []byte
	var res *stdHttp.Response
	var req *stdHttp.Request
	var err error

	url := getArgString(l, 1, luaNameHTTPGetFn)
	req, err = stdHttp.NewRequest("GET", url, nil)
	if err != nil {
		goto errh
	}

	// optionally push headers
	if l.ToValue(2) != nil {
		l.PushNil()
		for l.Next(2) {
			k := lua.CheckString(l, -2)
			v := lua.CheckString(l, -1)
			req.Header.Add(k, v)
			l.Pop(1)
		}
	}

	if res, err = stdHttp.DefaultClient.Do(req); err != nil {
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
	affinity := getOptionalArgInt(l, 4, -1, luaNameHTTPPostFn)
	sandbox := getStateSandbox(l)

	if sandbox.http == nil {
		if err := sandbox.initHTTP(); err != nil {
			lua.Errorf(l, "http initialization error: %s", err)
			panic("unreachable")
		}
	}

	// Avoid resource contention.
	// If http errors goroutine is trying to acquire this lock to call on_http_error lua fn
	// and the http requests channel is full, Post will block and thus create a deadlock
	sandbox.luaLock.Unlock()
	defer sandbox.luaLock.Lock()
	err := sandbox.http.Post(url, payload, contentType, affinity)
	if err != nil {
		lua.Errorf(l, "%s", err)
		panic("unreachable")
	}
	return 0
}

func (l *Sandbox) setHTTPTimeout(timeoutStr string) (err error) {
	var timeout time.Duration
	if timeout, err = time.ParseDuration(timeoutStr); err != nil {
		return
	}
	l.httpConfig.Timeout = timeout
	return
}

func (l *Sandbox) setHTTPChannelBuffer(c int) (err error) {
	l.httpConfig.ChanBuffer = c
	if l.http != nil {
		if err = l.http.Init(l.httpConfig, l.httpErrors); err != nil {
			return
		}
	}
	return
}

func (l *Sandbox) setHTTPConcurrency(c int) (err error) {
	l.httpConfig.Concurrency = c
	if l.http != nil {
		if err = l.http.Init(l.httpConfig, l.httpErrors); err != nil {
			return
		}
	}
	return
}

func (l *Sandbox) callOnHTTPError(e http.Error) {
	l.luaLock.Lock()
	defer l.luaLock.Unlock()

	l.state.Global(luaNameLogdModule)
	defer l.state.Pop(1)

	l.state.Field(-1, luaNameOnHTTPErrorFn)
	if !l.state.IsFunction(-1) {
		l.state.Pop(1)
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
