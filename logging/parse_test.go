package logging

import (
	"fmt"
	"log"
	"strings"
	"testing"
)

const log1 = "2017-09-07 14:54:39,474	DEBUG	[pool-5-thread-6]	control.RaptorHandler	PublisherCreateRequest: flow: Publish, step: Attempt, operation: CreatePublisher, traceId: Publish:Rumor:012ae1a5-3416-4458-b0c1-6eb3e0ab4c80\n"
const log2 = "2017-09-07 14:54:39,474	DEBUG	[pool-5-thread-6]	control.RaptorHandler	sessionId: 1_MX4xMDB-fjE1MDQ4MjEyNzAxMjR-WThtTVpEN0J2c1Z2TlJGcndTN1lpTExGfn4, flow: Publish, conId: connectionId: f41973e5-b27c-49e4-bcaf-1d48b153683e, step: Attempt, publisherId: b4da82c4-cac5-4e13-b1dc-bb1f42b475dd, fromAddress: f41973e5-b27c-49e4-bcaf-1d48b153683e, projectId: 100, operation: CreatePublisher, traceId: Publish:Rumor:112ae1a5-3416-4458-b0c1-6eb3e0ab4c80, streamId: b4da82c4-cac5-4e13-b1dc-bb1f42b475dd, remoteIpAddress: 127.0.0.1, correlationId: b90232b5-3ee5-4c65-bb4e-29286d6a2771\n"
const log3 = "2017-04-19 18:01:11,437     INFO [Test worker]    core.InstrumentationListener	i do not want to log anything special here\n"
const log4 = "2017-04-19 18:01:11,437     INFO [Test worker]    core.InstrumentationListener	only: one\n"
const log5 = "2017-11-16 19:07:56,883	WARN	[-]	-	flow: UpdateClientActivity, operation: HandleActiveEvent, step: Failure, luaRocks: true\n"
const log6 = "[2017-12-05T15:09:09.858] [WARN] main - flow: , operation: closePage, step: Failure, logLevel: WARN, url: https://10.1.6.113:6060/index.html?id=5NZM0okZ&wsUri=wss%3A%2F%2F10.1.6.113%3A6060%2Fws%3Fid%3D5NZM0okZ, err: Error  Protocol error (Target.closeTarget)  Target closed.     at Connection._onClose (/home/ernestrc/src/tbsip/node_modules/puppeteer/lib/Connection.js 124 23)     at emitTwo (events.js 125 13)     at WebSocket.emit (events.js 213 7)     at WebSocket.emitClose (/home/ernestrc/src/tbsip/node_modules/ws/lib/WebSocket.js 213 10)     at _receiver.cleanup (/home/ernestrc/src/tbsip/node_modules/ws/lib/WebSocket.js 195 41)     at Receiver.cleanup (/home/ernestrc/src/tbsip/node_modules/ws/lib/Receiver.js 520 15)     at WebSocket.finalize (/home/ernestrc/src/tbsip/node_modules/ws/lib/WebSocket.js 195 22)     at emitNone (events.js 110 20)     at Socket.emit (events.js 207 7)     at endReadableNT (_stream_readable.js 1047 12), class: Endpoint, id: 5NZM0okZ, timestamp: 1512515349858, duration: 2.017655998468399\n"

var expected1 = Log{
	time:    "14:54:39,474",
	date:    "2017-09-07",
	Level:   Debug,
	Thread:  "pool-5-thread-6",
	Class:   "control.RaptorHandler",
	Message: "",
	props: []Property{
		{"callType", "PublisherCreateRequest"},
		{"flow", "Publish"},
		{"step", "Attempt"},
		{"operation", "CreatePublisher"},
		{"traceId", "Publish:Rumor:012ae1a5-3416-4458-b0c1-6eb3e0ab4c80"},
	},
}

var expected2 = Log{
	date:    "2017-09-07",
	time:    "14:54:39,474",
	Level:   Debug,
	Thread:  "pool-5-thread-6",
	Class:   "control.RaptorHandler",
	Message: "",
	props: []Property{
		{"sessionId", "1_MX4xMDB-fjE1MDQ4MjEyNzAxMjR-WThtTVpEN0J2c1Z2TlJGcndTN1lpTExGfn4"},
		{"flow", "Publish"},
		{"connectionId", "f41973e5-b27c-49e4-bcaf-1d48b153683e"},
		{"step", "Attempt"},
		{"publisherId", "b4da82c4-cac5-4e13-b1dc-bb1f42b475dd"},
		{"fromAddress", "f41973e5-b27c-49e4-bcaf-1d48b153683e"},
		{"projectId", "100"},
		{"operation", "CreatePublisher"},
		{"traceId", "Publish:Rumor:112ae1a5-3416-4458-b0c1-6eb3e0ab4c80"},
		{"streamId", "b4da82c4-cac5-4e13-b1dc-bb1f42b475dd"},
		{"remoteIpAddress", "127.0.0.1"},
		{"correlationId", "b90232b5-3ee5-4c65-bb4e-29286d6a2771"},
	},
}

var expected3 = Log{
	date:    "2017-04-19",
	time:    "18:01:11,437",
	Level:   Info,
	Thread:  "Test worker",
	Class:   "core.InstrumentationListener",
	Message: "i do not want to log anything special here",
	props:   nil,
}

var expected4 = Log{
	date:    "2017-04-19",
	time:    "18:01:11,437",
	Level:   Info,
	Thread:  "Test worker",
	Class:   "core.InstrumentationListener",
	Message: "one",
	props:   []Property{{"callType", "only"}},
}

var expected5 = Log{
	date:    "2017-11-16",
	time:    "19:07:56,883",
	Level:   Warn,
	Thread:  "-",
	Class:   "-",
	Message: "",
	props: []Property{
		{"flow", "UpdateClientActivity"},
		{"operation", "HandleActiveEvent"},
		{"step", "Failure"},
		{"luaRocks", "true"},
	},
}

var expected6 = Log{
	time:   "15:09:09.858",
	date:   "2017-12-05",
	Level:  Warn,
	Thread: "main",
	Class:  "-",
	props: []Property{
		{"flow", ""},
		{"operation", "closePage"},
		{"step", "Failure"},
		{"logLevel", "WARN"},
		{"url", "https://10.1.6.113:6060/index.html?id=5NZM0okZ&wsUri=wss%3A%2F%2F10.1.6.113%3A6060%2Fws%3Fid%3D5NZM0okZ"},
		{"err", "Error  Protocol error (Target.closeTarget)  Target closed.     at Connection._onClose (/home/ernestrc/src/tbsip/node_modules/puppeteer/lib/Connection.js 124 23)     at emitTwo (events.js 125 13)     at WebSocket.emit (events.js 213 7)     at WebSocket.emitClose (/home/ernestrc/src/tbsip/node_modules/ws/lib/WebSocket.js 213 10)     at _receiver.cleanup (/home/ernestrc/src/tbsip/node_modules/ws/lib/WebSocket.js 195 41)     at Receiver.cleanup (/home/ernestrc/src/tbsip/node_modules/ws/lib/Receiver.js 520 15)     at WebSocket.finalize (/home/ernestrc/src/tbsip/node_modules/ws/lib/WebSocket.js 195 22)     at emitNone (events.js 110 20)     at Socket.emit (events.js 207 7)     at endReadableNT (_stream_readable.js 1047 12)"},
		{"class", "Endpoint"},
		{"id", "5NZM0okZ"},
		{"timestamp", "1512515349858"},
		{"duration", "2.017655998468399"},
	},
}

func testEquals(t *testing.T, output Log, expected Log) {
	if expected.Timestamp() != output.Timestamp() {
		t.Errorf("expected Time '%s' found '%s'", expected.Timestamp(), output.Timestamp())
	}
	if expected.Level != output.Level {
		t.Errorf("expected Level '%s' found '%s'", expected.Level, output.Level)
	}
	if expected.Thread != output.Thread {
		t.Errorf("expected Thread '%s' found '%s'", expected.Thread, output.Thread)
	}
	if expected.Class != output.Class {
		t.Errorf("expected Class '%s' found '%s'", expected.Class, output.Class)
	}
	if expected.Message != output.Message {
		t.Errorf("expected Message '%s' found '%s'", expected.Message, output.Message)
	}

	if len(expected.props) != len(output.props) {
		t.Errorf("expected output props len to be %d but found %d", len(expected.props), len(output.props))
	}

	seen := make(map[string]struct{})

	for i, p := range expected.props {
		k := p.key
		v := p.value
		if i >= len(output.props) || v != output.props[i].value {
			var out string
			if len(output.props) > i {
				out = output.props[i].value
			}
			t.Errorf("expected '%s' to be '%s'; found '%s'", k, v, out)
		}
		seen[k] = struct{}{}
	}

	for _, p := range output.props {
		k := p.key
		if _, ok := seen[k]; !ok {
			t.Errorf("unexpected key %s", k)
		}
	}
}

func TestParserProps1(t *testing.T) {
	logs := Parse(log1)
	testEquals(t, logs[0], expected1)
}

func TestParserProps2(t *testing.T) {
	logs := Parse(log2)
	testEquals(t, logs[0], expected2)
}

func TestParserProps5(t *testing.T) {
	logs := Parse(log5)
	testEquals(t, logs[0], expected5)
}

func TestParserProps6(t *testing.T) {
	logs := Parse(log6)
	testEquals(t, logs[0], expected6)
}

func TestParserNothing(t *testing.T) {
	logs := Parse(log3)
	testEquals(t, logs[0], expected3)
}

func TestParserOnlyCallTypeMsg(t *testing.T) {
	logs := Parse(log4)
	testEquals(t, logs[0], expected4)
}

func TestParserMulti(t *testing.T) {
	input := fmt.Sprintf("%s%s%s", log1, log3, log2)
	expected := []Log{
		expected1, expected3, expected2,
	}

	logs := Parse(input)
	for i, log := range logs {
		testEquals(t, log, expected[i])
	}
}

func TestParserChunks(t *testing.T) {
	input := fmt.Sprintf("%s%s%s", log1, log3, log2)
	expected := []Log{
		expected1, expected3, expected2,
	}

	chunks := make([]string, 3)
	chunks[0] = input[:50]
	chunks[1] = input[50:140]
	chunks[2] = input[140:]

	p := NewParser()
	output := make([]Log, 0)

	for _, chunk := range chunks {
		output = p.Parse(chunk, output)
	}

	for i, log := range output {
		testEquals(t, log, expected[i])
	}
}

func testGetProp(t *testing.T, log *Log, key, value string) {
	if actual, ok := log.Get(key); !ok || strings.Compare(actual, value) != 0 {
		t.Errorf("expected get prop '%s' to be '%s' but found '%s'", key, value, actual)
	}
}

func TestGetProp(t *testing.T) {
	parsed2 := Parse(log2)[0]
	testGetProp(t, &parsed2, "sessionId", "1_MX4xMDB-fjE1MDQ4MjEyNzAxMjR-WThtTVpEN0J2c1Z2TlJGcndTN1lpTExGfn4")
	testGetProp(t, &parsed2, "connectionId", "f41973e5-b27c-49e4-bcaf-1d48b153683e")
	testGetProp(t, &parsed2, "publisherId", "b4da82c4-cac5-4e13-b1dc-bb1f42b475dd")
	testGetProp(t, &parsed2, "fromAddress", "f41973e5-b27c-49e4-bcaf-1d48b153683e")
	testGetProp(t, &parsed2, "projectId", "100")
	testGetProp(t, &parsed2, "streamId", "b4da82c4-cac5-4e13-b1dc-bb1f42b475dd")
	testGetProp(t, &parsed2, "remoteIpAddress", "127.0.0.1")
	testGetProp(t, &parsed2, "correlationId", "b90232b5-3ee5-4c65-bb4e-29286d6a2771")

	parsed1 := Parse(log1)[0]
	testGetProp(t, &parsed1, "callType", "PublisherCreateRequest")
	testGetProp(t, &parsed1, "flow", "Publish")
}

// 3 log lines
func BenchmarkParserSmallInput(b *testing.B) {
	input := fmt.Sprintf("%s%s%s", log1, log3, log2)
	for i := 0; i < b.N; i++ {
		Parse(input)
	}
}

var benchBigInput = fmt.Sprintf("%s%s%s", log1, log3, log2)

// 100MB worth of log lines
const bigInputSize = 1000 * 1000 * 100

func init() {
	for len([]byte(benchBigInput)) < bigInputSize {
		benchBigInput = fmt.Sprintf("%s%s", benchBigInput, benchBigInput)
	}
	n := strings.Count(benchBigInput, "\n")
	log.Printf("Benchmark big input=%d bytes; logs=%d\n", len([]byte(benchBigInput)), n)
}

func BenchmarkParserBigInput(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Parse(benchBigInput)
	}
}
