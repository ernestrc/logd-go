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

var tick = flag.Int("T", 1000, "duration of input buffering period in Microseconds. Default is 1000us")
var script = flag.String("S", "", "Lua script that implements function log (time, level, flow, operation, step, properties). Defaults to printing a digest of the logs")
var scriptMode = flag.String("M", "bt", "Lua script load mode which controls whether the chunk can be text or binary (that is, a precompiled chunk). It may be the string 'b' (only binary chunks), 't' (only text chunks), or 'bt' (both binary and text). The default is 'bt'.")

func loadLua() *lua.State {
	l := lua.NewState()
	lua.OpenLibraries(l)

	var err error
	if *script == "" {
		err = lua.LoadString(l, `
		function log (time, level, flow, operation, step, properties)
		  print(string.format("%s\t%s\t%s", time, level, flow, operation, step))
		end`)
	} else {
		err = lua.LoadFile(l, *script, *scriptMode)
	}
	if err != nil {
		panic(err)
	}

	l.SetGlobal("log")
	return l
}

func main() {
	flag.Parse()

	tick := time.Duration(*tick * 1000)
	p := logging.NewParser()
	reader := bufio.NewReader(os.Stdin)
	ticker := time.NewTicker(tick)
	logs := make([]logging.Log, 0)
	l := loadLua()
	var buf [64 * 1000 * 1000]byte
	for {
		// time-based input buffering
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
