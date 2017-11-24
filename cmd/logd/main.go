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

var scriptFlag = flag.String("R", "", "Run Lua script processing pipeline")
var benchFlag = flag.String("B", "", "Benchmark Lua script processing pipeline")
var fullBenchFlag = flag.String("F", "", "Benchmark full processing pipeline (log parsing + lua processing)")
var fileFlag = flag.String("f", "/dev/stdin", "File to read data from")
var cpuProfileFlag = flag.String("p", "", "write cpu profile to file")
var memProfileFlag = flag.String("m", "", "write mem profile to file on SIGUSR2")
var debugFlag = flag.Bool("d", false, fmt.Sprintf("start a pprof server at %s", pprofServer))

// TODO add monitor directory flag fsnotify

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
	if f.Name == "F" || f.Name == "B" || f.Name == "R" {
		return
	}
	printFlag(f)
}

func printCommands(f *flag.Flag) {
	if f.Name != "F" && f.Name != "B" && f.Name != "R" {
		return
	}
	printFlag(f)
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n\n")
		fmt.Fprintf(os.Stderr, "\t%s (-S|-R|-B) <lua> [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nCommands:\n\n")
		flag.VisitAll(printCommands)
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
	flag.Usage()
	os.Exit(1)
}

func validateFlags() string {
	if *scriptFlag == "" && *benchFlag == "" && *fullBenchFlag == "" {
		exitError(fmt.Errorf("no lua script provided"))
	}
	if (*scriptFlag != "" && *benchFlag != "") ||
		(*fullBenchFlag != "" && *benchFlag != "") ||
		(*fullBenchFlag != "" && *scriptFlag != "") {
		exitError(fmt.Errorf("only one mode is allowed"))
	}
	if *scriptFlag != "" {
		return *scriptFlag
	}

	if *fullBenchFlag != "" {
		return *fullBenchFlag
	}

	return *benchFlag
}

func getReader() io.Reader {
	reader, err := os.Open(*fileFlag)
	if err != nil {
		exitError(err)
	}
	return bufio.NewReader(reader)
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

	flag.Parse()
	script := validateFlags()

	if f := cpuProfile(); f != nil {
		if err = pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintf(os.Stderr, "could not start cpu profile: %v", err)
		}
		defer pprof.StopCPUProfile()
	}

	l, err = lua.NewSandbox(script, nil)
	if err != nil {
		exitError(err)
	}
	defer l.Close()

	reader := getReader()
	exit := make(chan error)
	defer close(exit)

	if *scriptFlag != "" {
		go runPipeline(l, exit, reader)
	} else if *benchFlag != "" {
		go runLuaBench(l, exit, reader)
	} else {
		go runFullBench(l, exit, reader)
	}

	signals := make(chan os.Signal)
	defer close(signals)

	go sigHandler(l, script, exit, signals)
	signal.Notify(signals, syscall.SIGUSR1, syscall.SIGUSR2)

	if *debugFlag {
		go runPprofServer(exit)
	}

	err = <-exit
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		os.Exit(1)
	}
}
