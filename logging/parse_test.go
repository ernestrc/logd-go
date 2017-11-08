package logging

import (
	"fmt"
	"log"
	"strings"
	"testing"
)

const log1 = "2017-09-07 14:54:39,474	DEBUG	[pool-5-thread-6]	control.RaptorHandler	PublisherCreateRequest: flow: Publish, step: Attempt, operation: CreatePublisher, traceId: Publish:Rumor:012ae1a5-3416-4458-b0c1-6eb3e0ab4c80\n"
const log2 = "2017-09-07 14:54:39,474	DEBUG	[pool-5-thread-6]	control.RaptorHandler	PublisherCreateRequest: sessionId: 1_MX4xMDB-fjE1MDQ4MjEyNzAxMjR-WThtTVpEN0J2c1Z2TlJGcndTN1lpTExGfn4, flow: Publish, connectionId: f41973e5-b27c-49e4-bcaf-1d48b153683e, step: Attempt, publisherId: b4da82c4-cac5-4e13-b1dc-bb1f42b475dd, fromAddress: f41973e5-b27c-49e4-bcaf-1d48b153683e, projectId: 100, operation: CreatePublisher, traceId: Publish:Rumor:112ae1a5-3416-4458-b0c1-6eb3e0ab4c80, streamId: b4da82c4-cac5-4e13-b1dc-bb1f42b475dd, remoteIpAddress: 127.0.0.1, correlationId: b90232b5-3ee5-4c65-bb4e-29286d6a2771\n"
const log3 = "2017-04-19 18:01:11,437     INFO [Test worker]    core.InstrumentationListener   DebugCallType \n"

var expected1 = Log{
	time:      "14:54:39,474",
	date:      "2017-09-07",
	Level:     Debug,
	Thread:    "[pool-5-thread-6]",
	Class:     "control.RaptorHandler",
	Flow:      "Publish",
	Operation: "CreatePublisher",
	TraceID:   "Publish:Rumor:012ae1a5-3416-4458-b0c1-6eb3e0ab4c80",
	Step:      Attempt,
	props:     nil,
}

var expected2 = Log{
	date:      "2017-09-07",
	time:      "14:54:39,474",
	Level:     Debug,
	Thread:    "[pool-5-thread-6]",
	Class:     "control.RaptorHandler",
	Flow:      "Publish",
	Operation: "CreatePublisher",
	TraceID:   "Publish:Rumor:112ae1a5-3416-4458-b0c1-6eb3e0ab4c80",
	Step:      Attempt,
	props: []Property{
		{"sessionId", "1_MX4xMDB-fjE1MDQ4MjEyNzAxMjR-WThtTVpEN0J2c1Z2TlJGcndTN1lpTExGfn4"},
		{"connectionId", "f41973e5-b27c-49e4-bcaf-1d48b153683e"},
		{"publisherId", "b4da82c4-cac5-4e13-b1dc-bb1f42b475dd"},
		{"fromAddress", "f41973e5-b27c-49e4-bcaf-1d48b153683e"},
		{"projectId", "100"},
		{"streamId", "b4da82c4-cac5-4e13-b1dc-bb1f42b475dd"},
		{"remoteIpAddress", "127.0.0.1"},
		{"correlationId", "b90232b5-3ee5-4c65-bb4e-29286d6a2771"},
	},
}

var expected3 = Log{
	date:      "2017-04-19",
	time:      "18:01:11,437",
	Level:     Info,
	Thread:    "[Test worker]",
	Class:     "core.InstrumentationListener",
	Flow:      "",
	Operation: "",
	TraceID:   "",
	Step:      "",
	props:     nil,
}

func testEquals(t *testing.T, output Log, expected Log) {
	if expected.Timestamp() != output.Timestamp() {
		t.Errorf("expected Time %s found %s", expected.Timestamp(), output.Timestamp())
	}
	if expected.Level != output.Level {
		t.Errorf("expected Level() %s found %s", expected.Level, output.Level)
	}
	if expected.Thread != output.Thread {
		t.Errorf("expected Thread() %s found %s", expected.Thread, output.Thread)
	}
	if expected.Class != output.Class {
		t.Errorf("expected Class() %s found %s", expected.Class, output.Class)
	}
	if expected.Flow != output.Flow {
		t.Errorf("expected Flow() %s found %s", expected.Flow, output.Flow)
	}
	if expected.Operation != output.Operation {
		t.Errorf("expected Operation() %s found %s", expected.Operation, output.Operation)
	}
	if expected.TraceID != output.TraceID {
		t.Errorf("expected TraceID() %s found %s", expected.TraceID, output.TraceID)
	}
	if expected.Step != output.Step {
		t.Errorf("expected Step() %s found %s", expected.Step, output.Step)
	}

	seen := make(map[string]struct{})

	for i, p := range expected.props {
		k := p.key
		v := p.value
		if v != output.props[i].value {
			t.Errorf("expected %s %s found %s", k, expected.props[i].value, output.props[i].value)
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

func TestParserNoProps(t *testing.T) {
	logs := Parse(log1)
	testEquals(t, logs[0], expected1)
}

func TestParserProps(t *testing.T) {
	logs := Parse(log2)
	testEquals(t, logs[0], expected2)
}

func TestParserNothing(t *testing.T) {
	logs := Parse(log3)
	testEquals(t, logs[0], expected3)
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
	log := Parse(log2)[0]
	testGetProp(t, &log, "sessionId", "1_MX4xMDB-fjE1MDQ4MjEyNzAxMjR-WThtTVpEN0J2c1Z2TlJGcndTN1lpTExGfn4")
	testGetProp(t, &log, "connectionId", "f41973e5-b27c-49e4-bcaf-1d48b153683e")
	testGetProp(t, &log, "publisherId", "b4da82c4-cac5-4e13-b1dc-bb1f42b475dd")
	testGetProp(t, &log, "fromAddress", "f41973e5-b27c-49e4-bcaf-1d48b153683e")
	testGetProp(t, &log, "projectId", "100")
	testGetProp(t, &log, "streamId", "b4da82c4-cac5-4e13-b1dc-bb1f42b475dd")
	testGetProp(t, &log, "remoteIpAddress", "127.0.0.1")
	testGetProp(t, &log, "correlationId", "b90232b5-3ee5-4c65-bb4e-29286d6a2771")

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
