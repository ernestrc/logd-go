package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"time"

	lua "github.com/Shopify/go-lua"
	"github.com/opentok/blue/logging"
)

var tick = flag.Int("T", 1000, "duration of input buffering period in Microseconds [default: 1000us]")

func main() {
	flag.Parse()

	tick := time.Duration(*tick * 1000)
	p := logging.NewParser()
	reader := bufio.NewReader(os.Stdin)
	ticker := time.NewTicker(tick)
	logs := make([]logging.Log, 0)

	l := lua.NewState()
	lua.OpenLibraries(l)
	// lua.Require(
	// lua.PackageOpen(l)
	if err := lua.LoadString(l, `
    function log (time, level, flow, operation, step, properties)
      print(l)
    end`); err != nil {
		panic(err)
	}

	var buf [64 * 1000 * 1000]byte
	for {
		<-ticker.C
		n, err := reader.Read(buf[:])
		if err != nil {
			log.Fatal(err)
		}
		logs = p.Parse(string(buf[:n]), logs)
		for _, log := range logs {
			l.Global("log")
			l.PushString(log.Time())
			l.PushString(log.Level())
			l.PushString(log.Flow())
			l.PushString(log.Operation())
			l.PushString(log.Step())
			l.PushUserData(log.Props())
			l.Call(6, 0)
		}
		logs = logs[:0]
	}
}
