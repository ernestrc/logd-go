package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

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

	var buf [64 * 1000 * 1000]byte
	for {
		<-ticker.C
		n, err := reader.Read(buf[:])
		if err != nil {
			log.Fatal(err)
		}
		logs = p.Parse(string(buf[:n]), logs)
		for _, l := range logs {
			fmt.Println(l)
		}
		logs = logs[:0]
	}
}
