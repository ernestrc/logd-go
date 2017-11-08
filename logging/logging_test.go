package logging

import (
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
	if v := log.Flow; v != "xxx" {
		t.Errorf("flow not correct: %+v", v)
	}

	log.Set("timestamp", "a b")
	if time, date, timestamp := log.time, log.date, log.Timestamp(); date != "a" || time != "b" || timestamp != "a b" {
		t.Errorf("timestamp not correct: %s", timestamp)
	}

	if l := len(log.Props()); l != 2 {
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

	log.Flow = "xxx"
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
	log.props = append(log.props, Property{key: "a", value: "1234"})

	if ok := log.Remove("a"); !ok || len(log.props) > 0 {
		t.Errorf("expected true/len=0 found %t/%d", ok, len(log.props))
	}

	if ok := log.Remove("a"); ok || len(log.props) > 0 {
		t.Errorf("expected false/len=0 found %t/%d", ok, len(log.props))
	}

	if removed := log.Remove("flow"); removed {
		t.Errorf("flow removed but it was empty")
	}

	log.Flow = "xxx"
	if removed := log.Remove("flow"); !removed || log.Flow != "" {
		t.Errorf("flow not removed: removed=%t/'%s'", removed, log.Flow)
	}

	log.date = "a"
	log.time = "b"
	if removed := log.Remove("timestamp"); !removed || log.time != "" || log.date != "" {
		t.Errorf("timestamp not removed: removed=%t/'%s\t%s'", removed, log.date, log.time)
	}
}

var serLog = NewLog()

func init() {
	serLog.Thread = "[1234]"
	serLog.Level = "INFO"
	serLog.Class = "com.my.package.Class"
	serLog.time = "111111,111"
	serLog.date = "2017-24-11"
	serLog.Flow = "myFlow"
	serLog.props = append(serLog.props, Property{key: "a", value: "1234"})
	serLog.props = append(serLog.props, Property{key: "b", value: "xxx"})
}

func TestLogString(t *testing.T) {
	expected := "2017-24-11 111111,111	INFO	[1234]	com.my.package.Class	flow: myFlow, operation: , step: , traceId: , a: 1234, b: xxx"

	if str := serLog.String(); str != expected {
		t.Errorf("expected '%s' found '%s'", expected, str)
	}
}

func TestLogJSON(t *testing.T) {
	expected := `{"timestamp": "2017-24-11 111111,111", "level": "INFO", "thread": "[1234]", "class": "com.my.package.Class", "flow": "myFlow", "operation": "", "step": "", "traceId": "", "a": "1234", "b": "xxx"}`

	if str := serLog.JSON(); str != expected {
		t.Errorf("expected '%s' found '%s'", expected, str)
	}
}
