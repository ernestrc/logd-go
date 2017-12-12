package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"

	"github.com/ernestrc/logd/logging"
	"github.com/ernestrc/logd/lua"
)

const pprofServer = "localhost:6060"

type dirFlagType []string

func (d *dirFlagType) String() (s string) {
	for _, v := range *d {
		s += v
	}
	return
}

func (d *dirFlagType) Set(v string) error {
	*d = append(*d, v)
	return nil
}

var benchFlag = flag.Bool("B", false, "Benchmark lua script")
var fullBenchFlag = flag.Bool("F", false, "Benchmark processing pipeline (log parsing + lua processing)")
var fileFlag = flag.String("f", "/dev/stdin", "File to read data from")
var cpuProfileFlag = flag.String("p", "", "write cpu profile to file")
var memProfileFlag = flag.String("m", "", "write mem profile to file on SIGUSR2")
var profServer = flag.Bool("s", false, fmt.Sprintf("start a pprof server at %s", pprofServer))
var dirFlag dirFlagType

func printFlag(f *flag.Flag) {
	s := fmt.Sprintf("\t-%s", f.Name)
	name, usage := flag.UnquoteUsage(f)
	if len(name) > 0 {
		s += " <" + name + ">"
	}
	s += "\n\t\t"
	s += usage
	if f.DefValue != "" {
		s += " [default: "
		s += f.DefValue
		s += "]"
	}
	fmt.Fprint(os.Stderr, s, "\n")
}

func printOptions(f *flag.Flag) {
	if strings.Contains(f.Name, "test") {
		return
	}
	printFlag(f)
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n\n")
		fmt.Fprintf(os.Stderr, "\t%s <script> [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n\n")
		flag.VisitAll(printOptions)
	}
}

func runPipeline(l *lua.Sandbox, exit chan<- error, reader io.Reader) {
	p := logging.NewParser()

	logs := make([]logging.Log, 0)

	var buf [64 * 1000 * 1000]byte
	var err error
	for {
		var n int
		if n, err = reader.Read(buf[:]); err != nil {
			break
		}

		logs = p.Parse(string(buf[:n]), logs)

		for _, log := range logs {
			if err = l.CallOnLog(&log); err != nil {
				break
			}
		}

		logs = logs[:0]
	}
	if err != nil && err != io.EOF {
		fmt.Fprint(os.Stderr, "error: ")
		exit <- err
	} else {
		exit <- nil
	}
}

func exitError(err error) {
	fmt.Fprintf(os.Stderr, "error: %s\n\n", err)
	os.Exit(1)
}

func validateFlags() {
	args := flag.Args()
	if len(args) == 0 {
		exitError(fmt.Errorf("no lua script provided"))
	}
	if *fullBenchFlag && *benchFlag {
		exitError(fmt.Errorf("only one bench mode is allowed"))
	}

	if len(dirFlag) != 0 && (*fullBenchFlag || *benchFlag) {
		exitError(fmt.Errorf("-d not allowd with bench mode"))
	}
}

type bufioReadCloser struct {
	bufio.Reader
}

func (b *bufioReadCloser) Close() error {
	return nil
}

func getReader() (io.ReadCloser, error) {
	if len(dirFlag) != 0 {
		reader, err := NewReader()
		if err != nil {
			panic(err)
		}
		for _, dir := range dirFlag {
			if err := reader.Watch(dir); err != nil {
				return nil, err
			}
		}
		return reader, nil
	}
	reader, err := os.Open(*fileFlag)
	if err != nil {
		exitError(err)
	}
	return &bufioReadCloser{*bufio.NewReader(reader)}, nil
}

func createProfileFile(name string) *os.File {
	f, err := os.Create(name)
	if err != nil {
		exitError(fmt.Errorf("error creating profile file %s: %v", name, err))
	}
	return f
}

func cpuProfile() *os.File {
	if *cpuProfileFlag == "" {
		return nil
	}

	return createProfileFile(*cpuProfileFlag)
}

func memProfile() *os.File {
	if *memProfileFlag == "" {
		return nil
	}
	return createProfileFile(*memProfileFlag)
}

func sigHandler(l *lua.Sandbox, script string, exit chan error, sig chan os.Signal) {
	for sig := range sig {
		fmt.Fprintf(os.Stderr, "received: %s\n", sig)
		switch sig {
		case syscall.SIGUSR2:
			if f := memProfile(); f != nil {
				defer f.Close()
				runtime.GC()
				if err := pprof.WriteHeapProfile(f); err != nil {
					exit <- err
				}
			}
		case syscall.SIGUSR1:
			// reload lua state
			l.Init(script, nil)
		default:
			exit <- nil
		}
	}
}

func runPprofServer(exit chan error) {
	if err := http.ListenAndServe(pprofServer, nil); err != nil {
		exit <- err
	}
}

func main() {
	var err error
	var l *lua.Sandbox

	flag.Var(&dirFlag, "d", "Monitor directory recursively, ingesting all the new data written to files. Overrides -f flag")
	flag.Parse()
	validateFlags()
	script := flag.Args()[0]

	if f := cpuProfile(); f != nil {
		if err = pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "could not start cpu profile: %v\n", err)
		}
		defer pprof.StopCPUProfile()
	}

	l, err = lua.NewSandbox(script, nil)
	if err != nil {
		exitError(err)
	}
	defer l.Close()

	exit := make(chan error)
	defer close(exit)

	readeCloser, err := getReader()
	if err != nil {
		exitError(err)
	}
	defer readeCloser.Close()

	if *fullBenchFlag {
		go runFullBench(l, exit, readeCloser)
	} else if *benchFlag {
		go runLuaBench(l, exit, readeCloser)
	} else {
		go runPipeline(l, exit, readeCloser)
	}

	signals := make(chan os.Signal)
	defer close(signals)

	go sigHandler(l, script, exit, signals)
	signal.Notify(signals, syscall.SIGUSR1, syscall.SIGUSR2)

	if *profServer {
		go runPprofServer(exit)
	}

	err = <-exit
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
