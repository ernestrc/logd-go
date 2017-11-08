package logging

import "testing"

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
}

func TestLogString(t *testing.T) {
	log := NewLog()
	log.thread = "[1234]"
	log.level = "INFO"
	log.class = "com.my.package.Class"
	log.time = "111111,111"
	log.date = "2017-24-11"
	log.props = append(log.props, Property{key: "a", value: "1234"})
	log.props = append(log.props, Property{key: "b", value: "xxx"})

	expected := "2017-24-11	111111,111	INFO	[1234]	com.my.package.Class	a: 1234, b: xxx"

	if str := log.String(); str != expected {
		t.Errorf("expected '%s' found '%s'", expected, str)
	}
}
