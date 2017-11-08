package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/opentok/blue/logging"
)

type fwriter interface {
	io.Writer
	Flush() error
}

type logWriter func(io.Writer, *logging.Log)

var validOutputModes = [3]string{"stdout", "stderr", "null"}
var validOutputFormats = [2]string{"JSON", "otlog"}

var script = flag.String("S", "", "Lua script that implements transformation function map (logptr)")
var scriptMode = flag.String("M", "bt", "Lua script load mode which controls whether the chunk can be text or binary (that is, a precompiled chunk). It may be the string 'b' (only binary chunks), 't' (only text chunks), or 'bt' (both binary and text). The default is 'bt'")
var outputFormat = flag.String("F", "otlog", fmt.Sprintf("Output format of the logs. Default is 'otlog' which the same format than the input of the log. Other valid formats: %v", validOutputFormats))
var output = flag.String("O", "stdout", fmt.Sprintf("Output mode. Available modes: %v. Default is 'stdout'", validOutputModes))

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

func main() {
	flag.Parse()

	p := logging.NewParser()
	reader := bufio.NewReader(os.Stdin)
	writer := getLogWriter()
	ioWriter := getIoWriter()
	defer ioWriter.Flush()
	logs := make([]logging.Log, 0)
	l := loadLuaRuntime()

	var buf [64 * 1000 * 1000]byte
	for {
		n, err := reader.Read(buf[:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s", err)
			break
		}
		logs = p.Parse(string(buf[:n]), logs)
		for _, log := range logs {
			l.Global("map")
			l.PushUserData(&log)
			l.Call(1, 1)
			ptr := popLogPtr(l, 1, "map")
			if ptr != nil {
				writer(ioWriter, ptr)
			}
		}
		logs = logs[:0]
	}
}
