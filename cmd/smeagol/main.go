package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	lua "github.com/Shopify/go-lua"
	"github.com/opentok/blue/logging"
)

type fwriter interface {
	io.Writer
	Flush() error
}

type logWriter func(io.Writer, *logging.Log)

var validOutputModes = [3]string{"stdout", "stderr", "null"}
var validOutputFormats = [2]string{"JSON", "otlog"}

var script = flag.String("S", "", "Lua script to run")
var outputFormat = flag.String("F", "otlog", fmt.Sprintf("Output format of the logs. Default is 'otlog' which the same format than the input of the log. Other valid formats: %v", validOutputFormats))
var output = flag.String("O", "stdout", fmt.Sprintf("Output mode. Available modes: %v. Default is 'stdout'", validOutputModes))
var period = flag.Int("P", 0, "coroutine period in milliseconds. Default is 0 (deactivated)")

func getIoWriter() (o fwriter) {
	switch *output {
	case "stdout":
		o = NewLogWriter(os.Stdout)
	case "stderr":
		o = NewLogWriter(os.Stderr)
	case "null":
		o = NewLogWriter(ioutil.Discard)
	default:
		fmt.Fprintf(os.Stderr, "only %v are allowd as modes", validOutputModes)
		os.Exit(1)
	}
	return
}

func logWrite(o io.Writer, log *logging.Log) {
	o.Write([]byte(log.String()))
}

func jsonWrite(o io.Writer, log *logging.Log) {
	b := log.JSON()
	o.Write([]byte(b))
}

func getLogWriter() logWriter {
	switch *outputFormat {
	case "otlog":
		return logWrite
	case "JSON", "json":
		return jsonWrite
	default:
		fmt.Fprintf(os.Stderr, "invalid output format '%s'. Available: %v", *outputFormat, validOutputFormats)
		os.Exit(1)
	}

	return nil
}

func runTicker(luaLock *sync.Mutex, l *lua.State, exit chan<- error) {
	// deactivated if period is set to <= 0
	if *period <= 0 {
		return
	}
	ticker := time.NewTicker(time.Duration(*period * 1000 * 1000))
	defer ticker.Stop()

	for {
		luaLock.Lock()
		defined := callLuaScheduledFn(l)
		luaLock.Unlock()
		// stop goroutine if on_tick is not defined
		if !defined {
			fmt.Fprintf(os.Stderr, "on_tick is not defined. coroutine deactivated")
			return
		}
		<-ticker.C
	}
}

func runPipeline(luaLock *sync.Mutex, l *lua.State, exit chan<- error, reader io.Reader, writer logWriter, ioWriter fwriter) {
	p := logging.NewParser()

	logs := make([]logging.Log, 0)

	var buf [64 * 1000 * 1000]byte
	var err error
	for {
		var n int
		if n, err = reader.Read(buf[:]); err != nil {
			break
		}

		luaLock.Lock()
		logs = p.Parse(string(buf[:n]), logs)

		for _, log := range logs {
			if ptr := callLuaMapFn(l, &log); ptr != nil {
				writer(ioWriter, ptr)
			}
		}

		logs = logs[:0]
		luaLock.Unlock()
	}
	if err != nil && err != io.EOF {
		fmt.Fprint(os.Stderr, "error: ")
		exit <- err
	} else {
		fmt.Fprint(os.Stderr, "EOF")
		exit <- nil
	}
}

func main() {
	flag.Parse()
	writer := getLogWriter()
	reader := bufio.NewReader(os.Stdin)
	ioWriter := getIoWriter()
	l := loadLuaRuntime()
	exit := make(chan error)
	var luaMutex sync.Mutex

	defer ioWriter.Flush()
	go runTicker(&luaMutex, l, exit)
	go runPipeline(&luaMutex, l, exit, reader, writer, ioWriter)

	err := <-exit
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
}
