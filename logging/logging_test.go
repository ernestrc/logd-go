package logging

import (
	"bytes"
	"testing"
)

func TestLogSet(t *testing.T) {
	log := NewLog()

	log.Set("a", "1234")
	if v := log.props[0]; v.key != "a" || v.value != "1234" {
		t.Errorf("value not correct: %+v", v)
	}

	log.Set("b", "xxx")
	if v := log.props[1]; v.key != "b" || v.value != "xxx" {
		t.Errorf("value not correct: %+v", v)
	}

	log.Set("a", "xxx")
	if v := log.props[0]; v.key != "a" || v.value != "xxx" {
		t.Errorf("value not correct: %+v", v)
	}

	log.Set("flow", "xxx")
	if v, ok := log.Get("flow"); !ok || v != "xxx" {
		t.Errorf("flow not correct: %+v", v)
	}

	log.Set("timestamp", "a b")
	if time, date, timestamp := log.time, log.date, log.Timestamp(); date != "a" || time != "b" || timestamp != "a b" {
		t.Errorf("timestamp not correct: %s", timestamp)
	}

	if l := len(log.Props()); l != 3 {
		t.Errorf("expected length of properties to be 2 found: %d", l)
	}
}

func TestLogGet(t *testing.T) {
	log := NewLog()
	log.props = append(log.props, Property{key: "a", value: "1234"})

	if v, ok := log.Get("a"); v != "1234" || !ok {
		t.Errorf("expected '1234'/true found '%s'/%t", v, ok)
	}

	if v, ok := log.Get("b"); v != "" || ok {
		t.Errorf("expected ''/false found '%s'/%t", v, ok)
	}

	if v, ok := log.Get("flow"); ok || v != "" {
		t.Errorf("flow not correct: %+v", v)
	}

	log.Set("flow", "xxx")
	if v, ok := log.Get("flow"); !ok || v != "xxx" {
		t.Errorf("flow not correct: %+v", v)
	}

	log.date = "a"
	log.time = "b"
	if v, ok := log.Get("timestamp"); !ok || v != "a b" {
		t.Errorf("timestamp not correct: %+v", v)
	}
}

func TestLogRemove(t *testing.T) {
	log := NewLog()
	log.props = append(log.props, Property{key: "a", value: "1234"}, Property{key: "b", value: "0987"})

	if ok := log.Remove("a"); !ok || len(log.props) != 1 {
		t.Errorf("expected true/len=1 found %t/%d", ok, len(log.props))
	}

	if ok := log.Remove("b"); !ok || len(log.props) != 0 {
		t.Errorf("expected true/len=0 found %t/%d", ok, len(log.props))
	}

	if ok := log.Remove("a"); ok || len(log.props) != 0 {
		t.Errorf("expected false/len=0 found %t/%d", ok, len(log.props))
	}

	if removed := log.Remove("flow"); removed {
		t.Errorf("flow removed but it was empty")
	}

	log.Set("flow", "xxx")
	removed := log.Remove("flow")
	if flow, ok := log.Get("flow"); !removed || ok || flow != "" {
		t.Errorf("flow not removed: removed=%t/'%s'", removed, flow)
	}

	log.date = "a"
	log.time = "b"
	if removed := log.Remove("timestamp"); !removed || log.time != "" || log.date != "" {
		t.Errorf("timestamp not removed: removed=%t/'%s\t%s'", removed, log.date, log.time)
	}
}

var serLog = NewLog()
var serLog2 = NewLog()
var serLog3 = NewLog()
var serLog4 = NewLog()

func init() {
	serLog.Thread = ""
	serLog.Level = "INFO"
	serLog.time = "111111,111"
	serLog.date = "2017-24-11"
	serLog.Message = "my message"
	serLog.Set("flow", "myFlow")
	serLog.props = append(serLog.props, Property{key: "a", value: "1234"})
	serLog.props = append(serLog.props, Property{key: "b", value: "xxx"})

	serLog2.Thread = "1234"
	serLog2.Level = "INFO"
	serLog2.Class = "com.my.package.Class"
	serLog2.time = "111111,111"
	serLog2.date = "2017-24-11"
	serLog2.Message = "my message"

	serLog3.Thread = "1234"
	serLog3.Level = "INFO"
	serLog3.Class = "com.my.package.Class"
	serLog3.time = "111111,111"
	serLog3.date = "2017-24-11"
	serLog3.Set("flow", "myFlow")
	serLog3.props = append(serLog3.props, Property{key: "a", value: "1234\n\r\"\b"})
	serLog3.props = append(serLog3.props, Property{key: "b", value: "xxx"})

	serLog4.Thread = "1234"
	serLog4.Level = "INFO"
	serLog4.Class = "com.my.package.Class"
	serLog4.time = "111111,111"
	serLog4.date = "2017-24-11"
}

func TestLogSerialize(t *testing.T) {
	testCases := []struct {
		fn       func() string
		expected string
	}{
		{serLog.String, "2017-24-11 111111,111	INFO	[-]	-	flow: myFlow, a: 1234, b: xxx, msg: my message"},
		{serLog2.String, "2017-24-11 111111,111	INFO	[1234]	com.my.package.Class	msg: my message"},
		{serLog3.String, "2017-24-11 111111,111	INFO	[1234]	com.my.package.Class	flow: myFlow, a: 1234\\n\\r\\\"\\b, b: xxx"},
		{serLog4.String, "2017-24-11 111111,111	INFO	[1234]	com.my.package.Class"},
		{serLog.JSON, `{"timestamp": "2017-24-11 111111,111", "level": "INFO", "thread": "", "class": "", "flow": "myFlow", "a": "1234", "b": "xxx", "msg": "my message"}`},
		{serLog2.JSON, `{"timestamp": "2017-24-11 111111,111", "level": "INFO", "thread": "1234", "class": "com.my.package.Class", "msg": "my message"}`},
		{serLog3.JSON, `{"timestamp": "2017-24-11 111111,111", "level": "INFO", "thread": "1234", "class": "com.my.package.Class", "flow": "myFlow", "a": "1234\n\r\"\b", "b": "xxx"}`},
		{serLog4.JSON, `{"timestamp": "2017-24-11 111111,111", "level": "INFO", "thread": "1234", "class": "com.my.package.Class"}`},
	}

	for _, tcase := range testCases {
		if str := tcase.fn(); str != tcase.expected {
			t.Errorf("expected '%s' found '%s'", tcase.expected, str)
		}
	}
}

func BenchmarkLogString(t *testing.B) {
	for i := 0; i < t.N; i++ {
		serLog.String()
		serLog2.String()
		serLog3.String()
	}
}

func BenchmarkLogJSON(t *testing.B) {
	for i := 0; i < t.N; i++ {
		serLog.JSON()
		serLog2.JSON()
		serLog3.JSON()
	}
}

func BenchmarkLogWriteTo(t *testing.B) {
	var buf bytes.Buffer
	for i := 0; i < t.N; i++ {
		serLog.WriteTo(&buf)
		serLog2.WriteTo(&buf)
		serLog3.WriteTo(&buf)
	}
}

func BenchmarkLogWriteJsonTo(t *testing.B) {
	var buf bytes.Buffer
	for i := 0; i < t.N; i++ {
		serLog.WriteJSONTo(&buf)
		serLog2.WriteJSONTo(&buf)
		serLog3.WriteJSONTo(&buf)
	}
}
