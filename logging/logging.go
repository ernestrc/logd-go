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

// Property represents an arbitrary key-value pair in a Log
type Property struct {
	key   string
	value string
}

// Log represents a structured log
type Log struct {
	/* header */
	time   string
	date   string
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

// Get returns a the value of key set in the log properties or ok = false
// if there's no value set for that key
func (l *Log) Get(key string) (value string, ok bool) {
	for _, p := range l.props {
		if p.key == key {
			ok = true
			value = p.value
			return
		}
	}

	return
}

// Props returns a slice of the Log arbitrary key-value properties.
// It does not contain any of the named properties.
func (l *Log) Props() []Property {
	return l.props
}
