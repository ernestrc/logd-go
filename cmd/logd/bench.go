package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/ernestrc/logd/logging"
	"github.com/ernestrc/logd/lua"
)

var benchLogs = []logging.Log{}
var benchSandbox *lua.Sandbox
var benchParser *logging.Parser
var benchData []byte

func getTimeOp(res *testing.BenchmarkResult) (int64, string) {
	top := res.NsPerOp()
	unit := "ns/iter"
	if top >= 1000000 {
		top /= 1000000
		unit = "ms/iter"
	} else if top >= 1000 {
		top /= 1000
		unit = "us/iter"
	}

	return top, unit
}

func printRes(b []byte, res testing.BenchmarkResult) {
	bytesOp := res.AllocedBytesPerOp()
	allocsOp := res.AllocsPerOp()

	iterations := res.N
	totalLogs := iterations * len(benchLogs)
	top, unit := getTimeOp(&res)
	logsSec := float64(totalLogs) / res.T.Seconds()
	allocMb := (float64(res.MemBytes) / 1000000)
	mbSec := allocMb / res.T.Seconds()
	allocsSec := float64(res.MemAllocs) / res.T.Seconds()
	bytesIter := len(b)
	processedMb := (float64(bytesIter) / 1000000) * float64(iterations)
	processedMbSec := processedMb / res.T.Seconds()

	fmt.Fprintf(os.Stderr, "\n%30s\n", "Benchmark results")
	fmt.Fprintf(os.Stderr, "%30s\n", "---------------------------------------------")
	fmt.Fprintf(os.Stderr, "%20d\titerations\n%20d\tlogs\n%20.1f\tsec\n%20d\tallocs\n%20.1f\tallocated_MB\n%20.1f\tprocessed_MB\n\n",
		iterations, totalLogs, res.T.Seconds(), res.MemAllocs, allocMb, processedMb)
	fmt.Fprintf(os.Stderr, "%20d\tlogs/iter\n%20d\t%s\n%20d\talloc_bytes/iter\n%20d\tallocs/iter\n%20d\tprocessed_bytes/iter\n\n",
		len(benchLogs), top, unit, bytesOp, allocsOp, bytesIter)
	fmt.Fprintf(os.Stderr, "%20.1f\tlogs/sec\n%20.1f\tallocs/sec\n%20.1f\talloc_MB/sec\n%20.1f\tprocessed_MB/sec\n\n",
		logsSec, allocsSec, mbSec, processedMbSec)
}

func runLuaBench(l *lua.Sandbox, exit chan<- error, reader io.Reader) {
	p := logging.NewParser()
	benchSandbox = l

	b, err := ioutil.ReadAll(reader)
	if err != nil {
		exit <- err
		return
	}
	benchLogs = p.Parse(string(b), benchLogs)
	res := testing.Benchmark(benchLuaScriptOnly)
	printRes(b, res)
	exit <- nil
}

func runFullBench(l *lua.Sandbox, exit chan<- error, reader io.Reader) {
	var err error
	benchParser = logging.NewParser()
	benchSandbox = l

	benchData, err = ioutil.ReadAll(reader)
	if err != nil {
		exit <- err
		return
	}
	res := testing.Benchmark(benchFullPipeline)

	// parse logs so we can use n logs in calculations
	benchLogs = benchParser.Parse(string(benchData), benchLogs)
	printRes(benchData, res)

	exit <- nil
}

func benchFullPipeline(b *testing.B) {
	var err error
	for i := 0; i < b.N; i++ {
		benchLogs = benchParser.Parse(string(benchData), benchLogs)
		for _, log := range benchLogs {
			if err = benchSandbox.CallOnLog(&log); err != nil {
				b.Errorf("error while running benchmark: %v", err)
			}
		}
		benchLogs = benchLogs[:0]
	}
}

func benchLuaScriptOnly(b *testing.B) {
	var err error
	for i := 0; i < b.N; i++ {
		for _, log := range benchLogs {
			if err = benchSandbox.CallOnLog(&log); err != nil {
				b.Errorf("error while running benchmark: %v", err)
			}
		}
	}
}
