package logging

import (
	"fmt"
	"strings"
)

const (
	Info  string = "INFO"
	Debug        = "DEBUG"
	Trace        = "TRACE"
	Error        = "ERROR"
	Warn         = "WARN"
)

const (
	Attempt string = "Attempt"
	Success        = "Success"
	Failure        = "Failure"
	Event          = "Event"
)

const (
	KeyFlow      string = "flow"
	KeyOperation        = "operation"
	KeyStep             = "step"
	KeyTraceID          = "traceId"
	KeyThread           = "thread"
	KeyClass            = "class"
	KeyTime             = "time"
	KeyDate             = "date"
	KeyTimestamp        = "timestamp"
)

// Property represents an arbitrary key-value pair in a Log
type Property struct {
	key   string
	value string
}

// Log represents a structured log
type Log struct {
	/* header */
	date   string
	time   string
	Level  string `json:"level"`
	Thread string `json:"thread"`
	Class  string `json:"class"`

	/* named properties */
	TraceID   string `json:"traceId"`
	Flow      string `json:"flow"`
	Operation string `json:"operation"`
	Step      string `json:"step"`

	/* other properties */
	props []Property
}

// NewLog allocates storage and initializes a Log structure
func NewLog() *Log {
	l := new(Log)
	l.props = make([]Property, 0)
	return l
}

// Reset resets all log properties to their zero values
func (l *Log) Reset() {
	*l = Log{props: l.props[:0]}
}

// Timestamp returns the log timestamp
func (l *Log) Timestamp() string {
	return fmt.Sprintf("%s %s", l.date, l.time)
}

// Remove will remove the passed key from the log properties.
// It returns true if a property with the given key was found and removed.
func (l *Log) Remove(key string) (found bool) {
	switch key {
	case KeyFlow, KeyOperation, KeyStep, KeyTraceID, KeyThread, KeyClass, KeyTime, KeyDate, KeyTimestamp:
		found = l.Set(key, "")
	default:
		last := len(l.props) - 1
		if last < 0 {
			return
		}
		for i, p := range l.props {
			if p.key == key {
				found = true
				if i == last {
					l.props = l.props[:i]
					return
				}
				copy(l.props[i:], l.props[i+1:])
				l.props = l.props[:last]
				return
			}
		}
	}

	return
}

// Set will upsert the value of passed key in the log properties.
// It returns false if key was not found and property was added or true if it was upserted.
func (l *Log) Set(key string, value string) (upsert bool) {
	switch key {
	case KeyTimestamp:
		upsert = l.date != "" || l.time != ""
		// special case, set both time and date to zero values
		if value == "" {
			l.date = ""
			l.time = ""
			break
		}
		o := strings.Split(value, " ")
		if len(o) != 2 {
			panic(fmt.Sprintf("invalid timestamp format: %s", value))
		}
		l.date = o[0]
		l.time = o[1]
	case KeyFlow:
		upsert = l.Flow != ""
		l.Flow = value
	case KeyOperation:
		upsert = l.Operation != ""
		l.Operation = value
	case KeyStep:
		upsert = l.Step != ""
		l.Step = value
	case KeyTraceID:
		upsert = l.TraceID != ""
		l.TraceID = value
	case KeyThread:
		upsert = l.Thread != ""
		l.Thread = value
	case KeyClass:
		upsert = l.Class != ""
		l.Class = value
	case KeyTime:
		upsert = l.time != ""
		l.time = value
	case KeyDate:
		upsert = l.date != ""
		l.date = value
	default:
		for i, p := range l.props {
			if p.key == key {
				l.props[i].value = value
				upsert = true
				return
			}
		}
		l.props = append(l.props, Property{key: key, value: value})
	}
	return
}

// Get returns a the value of key set in the log properties or ok = false
// if there's no value set for that key
func (l *Log) Get(key string) (value string, ok bool) {
	switch key {
	case KeyTimestamp:
		value = l.Timestamp()
		ok = value != ""
	case KeyFlow:
		ok = l.Flow != ""
		value = l.Flow
	case KeyOperation:
		ok = l.Operation != ""
		value = l.Operation
	case KeyStep:
		ok = l.Step != ""
		value = l.Step
	case KeyTraceID:
		ok = l.TraceID != ""
		value = l.TraceID
	case KeyThread:
		ok = l.Thread != ""
		value = l.Thread
	case KeyClass:
		ok = l.Class != ""
		value = l.Class
	case KeyTime:
		ok = l.time != ""
		value = l.time
	case KeyDate:
		ok = l.date != ""
		value = l.date
	default:
		for _, p := range l.props {
			if p.key == key {
				ok = true
				value = p.value
				return
			}
		}
	}

	return
}

// Props returns a slice of the Log arbitrary key-value properties.
// It does not contain any of the named properties.
func (l *Log) Props() []Property {
	return l.props
}

func (l *Log) serialize(template, templateSep, keySep, valueSep string) (str string) {
	str = template
	if len(l.props) == 0 {
		return
	}

	first := l.props[0]
	str += templateSep
	str += first.key
	str += valueSep
	str += first.value

	for _, p := range l.props[1:] {
		str += keySep
		str += p.key
		str += valueSep
		str += p.value
	}

	return
}

func (l *Log) String() (str string) {
	str += fmt.Sprintf("%s %s\t%s\t%s\t%s", l.date, l.time, l.Level, l.Thread, l.Class)
	str = l.serialize(str, "\t", ", ", ": ")
	return
}

// JSON serializes the log in JSON format
func (l *Log) JSON() (str string) {
	str += fmt.Sprintf("{\"timestamp\": \"%s %s\", \"level\": \"%s\", \"thread\": \"%s\", \"class\": \"%s", l.date, l.time, l.Level, l.Thread, l.Class)
	str = l.serialize(str, `", "`, `", "`, `": "`)
	str += `"}`
	return str
}
