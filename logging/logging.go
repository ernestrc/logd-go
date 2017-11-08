package logging

import (
	"fmt"
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
	level  string
	thread string
	class  string

	/* named properties */
	traceID   string
	flow      string
	operation string
	step      string

	/* other properties */
	props []Property
}

// NewLog allocates storage and initializes a Log structure
func NewLog() *Log {
	l := new(Log)
	l.props = make([]Property, 0)
	return l
}

// Time returns the log timestamp
func (l *Log) Time() string {
	return fmt.Sprintf("%s %s", l.date, l.time)
}

// Level returns the log level
func (l *Log) Level() string {
	return l.level
}

// Thread returns the log thread
func (l *Log) Thread() string {
	return l.thread
}

// Class returns the log class
func (l *Log) Class() string {
	return l.class
}

// TraceID returns the log traceID
func (l *Log) TraceID() string {
	return l.traceID
}

// Flow returns the log flow
func (l *Log) Flow() string {
	return l.flow
}

// Operation returns the log operation
func (l *Log) Operation() string {
	return l.operation
}

// Step returns the log step
func (l *Log) Step() string {
	return l.step
}

// Remove will remove the passed key from the log properties.
// It returns true if a property with the given key was found and removed.
func (l *Log) Remove(key string) (found bool) {
	switch key {
	case KeyFlow, KeyOperation, KeyStep, KeyTraceID, KeyThread, KeyClass, KeyTime, KeyDate:
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
	case KeyFlow:
		upsert = l.flow != ""
		l.flow = value
	case KeyOperation:
		upsert = l.operation != ""
		l.operation = value
	case KeyStep:
		upsert = l.step != ""
		l.step = value
	case KeyTraceID:
		upsert = l.traceID != ""
		l.traceID = value
	case KeyThread:
		upsert = l.thread != ""
		l.thread = value
	case KeyClass:
		upsert = l.class != ""
		l.class = value
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
	case KeyFlow:
		ok = l.flow != ""
		value = l.flow
	case KeyOperation:
		ok = l.operation != ""
		value = l.operation
	case KeyStep:
		ok = l.step != ""
		value = l.step
	case KeyTraceID:
		ok = l.traceID != ""
		value = l.traceID
	case KeyThread:
		ok = l.thread != ""
		value = l.thread
	case KeyClass:
		ok = l.class != ""
		value = l.class
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

func (l *Log) String() (str string) {
	str += fmt.Sprintf("%s\t%s\t%s\t%s\t%s", l.date, l.time, l.level, l.thread, l.class)

	if len(l.props) == 0 {
		return
	}

	first := l.props[0]
	str += "\t"
	str += first.key
	str += ": "
	str += first.value

	for _, p := range l.props[1:] {
		str += ", "
		str += p.key
		str += ": "
		str += p.value
	}

	return
}
