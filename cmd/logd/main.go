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

	log "github.com/sirupsen/logrus"
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

// TODO check that bench is not being called with -r flag
var scriptFlag = flag.String("R", "", "Run Lua script processing pipeline")
var benchFlag = flag.String("B", "", "Benchmark Lua script processing pipeline")
var fullBenchFlag = flag.String("F", "", "Benchmark full processing pipeline (log parsing + lua processing)")
var fileFlag = flag.String("f", "/dev/stdin", "File to read data from")
var cpuProfileFlag = flag.String("p", "", "write cpu profile to file")
var memProfileFlag = flag.String("m", "", "write mem profile to file on SIGUSR2")
var logDebugLevel = flag.Bool("d", false, "enable process debugging logs")
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
	var n int

	if l.ProtectedMode() {
	protectedCall:
		for {
			if n, err = reader.Read(buf[:]); err != nil {
				break
			}
			logs = p.Parse(string(buf[:n]), logs)
			for _, log := range logs {
				if err = l.ProtectedCallOnLog(&log); err != nil {
					break protectedCall
				}
			}
			logs = logs[:0]
		}
	} else {
	call:
		for {
			if n, err = reader.Read(buf[:]); err != nil {
				break
			}

			logs = p.Parse(string(buf[:n]), logs)
			for _, log := range logs {
				if err = l.CallOnLog(&log); err != nil {
					break call
				}
			}
			logs = logs[:0]
		}
	}
	if err != nil && err != io.EOF {
		fmt.Fprint(os.Stderr, "error: ")
		exit <- err
	} else {
		exit <- nil
	}
}

func usageError(err error) {
	fmt.Fprintf(os.Stderr, "error: %s\n\n", err)
	flag.Usage()
	os.Exit(1)
}

func exitError(err error) {
	fmt.Fprintf(os.Stderr, "error: %s\n\n", err)
	os.Exit(1)
}

func validateFlags() string {
	if *scriptFlag == "" && *benchFlag == "" && *fullBenchFlag == "" {
		usageError(fmt.Errorf("no lua script provided"))
	}
	if (*scriptFlag != "" && *benchFlag != "") ||
		(*fullBenchFlag != "" && *benchFlag != "") ||
		(*fullBenchFlag != "" && *scriptFlag != "") {
		usageError(fmt.Errorf("only one mode is allowed"))
	}
	if *scriptFlag != "" {
		return *scriptFlag
	}

	if *fullBenchFlag != "" {
		return *fullBenchFlag
	}

	return *benchFlag
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

func enableCPUProfile() bool {
	if *cpuProfileFlag == "" {
		return false
	}

	f := createProfileFile(*cpuProfileFlag)

	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Fprintf(os.Stderr, "could not start cpu profile: %v\n", err)
		return false
	}

	return true
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
			if err := l.Init(script); err != nil {
				panic(err)
			}
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

// logd debug logging formatter for logrus
type logFormatter struct{}

func (f *logFormatter) Format(e *log.Entry) ([]byte, error) {
	log := logging.NewLog()
	log.Message = e.Message
	log.Level = e.Level.String()
	log.Set(logging.KeyTimestamp, e.Time.Format("2006-01-02 15:04:05.000"))

	for k, v := range e.Data {
		log.Set(k, fmt.Sprintf("%v", v))
	}

	return []byte(fmt.Sprintf("%s\n", log.String())), nil
}

func initLogging() {
	log.SetOutput(os.Stderr)
	log.SetFormatter(&logFormatter{})

	if *logDebugLevel {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.WarnLevel)
	}
}

func main() {
	var err error
	var l *lua.Sandbox

	flag.Var(&dirFlag, "r", "Monitor directory recursively, ingesting all the new data written to files. Overrides -f flag")
	flag.Parse()
	script := validateFlags()

	initLogging()

	if enableCPUProfile() {
		defer pprof.StopCPUProfile()
	}

	l, err = lua.NewSandbox(script)
	if err != nil {
		exitError(err)
	}
	defer l.Close()

	exit := make(chan error)
	defer close(exit)

	readCloser, err := getReader()
	if err != nil {
		exitError(err)
	}
	defer readCloser.Close()

	if *scriptFlag != "" {
		go runPipeline(l, exit, readCloser)
	} else if *benchFlag != "" {
		go runLuaBench(l, exit, readCloser)
	} else {
		go runFullBench(l, exit, readCloser)
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
