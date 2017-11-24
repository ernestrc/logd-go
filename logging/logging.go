package logging

import (
	"bytes"
	"fmt"
	"io"
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
	KeyThread    = "thread"
	KeyClass     = "class"
	KeyLevel     = "level"
	KeyTime      = "time"
	KeyDate      = "date"
	KeyTimestamp = "timestamp"
	KeyMessage   = "msg"
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
	Level  string
	Thread string
	Class  string

	/* other properties */
	Message string
	props   []Property
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
	case KeyThread, KeyLevel, KeyClass, KeyTime, KeyDate, KeyTimestamp, KeyMessage:
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
		// set both time and date to zero values
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
	case KeyLevel:
		upsert = l.Level != ""
		l.Level = value
	case KeyMessage:
		upsert = l.Message != ""
		l.Message = value
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
	case KeyThread:
		ok = l.Thread != ""
		value = l.Thread
	case KeyMessage:
		ok = l.Message != ""
		value = l.Message
	case KeyLevel:
		ok = l.Level != ""
		value = l.Level
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

func escape(buf *bytes.Buffer, r rune) {
	switch r {
	case '\b':
		buf.WriteByte('\\')
		buf.WriteByte('b')
	case '\f':
		buf.WriteByte('\\')
		buf.WriteByte('f')
	case '\n':
		buf.WriteByte('\\')
		buf.WriteByte('n')
	case '\r':
		buf.WriteByte('\\')
		buf.WriteByte('r')
	case '\t':
		buf.WriteByte('\\')
		buf.WriteByte('t')
	case '"':
		buf.WriteByte('\\')
		buf.WriteByte('"')
	case '\\':
		buf.WriteByte('\\')
		buf.WriteByte('\\')
	default:
		buf.WriteRune(r)
	}
}

func appendProp(keySep, valueSep string, p Property, buf *bytes.Buffer) {
	buf.WriteString(keySep)
	for _, r := range p.key {
		escape(buf, r)
	}
	buf.WriteString(valueSep)
	for _, r := range p.value {
		escape(buf, r)
	}
}

func (l *Log) serialize(buf *bytes.Buffer, headerSep, keySep, valueSep string) {
	lenProps := len(l.props)
	if lenProps == 0 {
		keySep = headerSep
		goto msg
	}

	appendProp(headerSep, valueSep, l.props[0], buf)

	for _, p := range l.props[1:] {
		appendProp(keySep, valueSep, p, buf)
	}

msg:
	if l.Message != "" {
		appendProp(keySep, valueSep, Property{key: KeyMessage, value: l.Message}, buf)
	}
}

func (l *Log) WriteJSONTo(buf *bytes.Buffer) {
	buf.WriteString("{\"timestamp\": \"")
	buf.WriteString(l.date)
	buf.WriteByte(' ')
	buf.WriteString(l.time)
	buf.WriteString(`", "level": "`)
	buf.WriteString(l.Level)
	buf.WriteString(`", "thread": "`)
	buf.WriteString(l.Thread)
	buf.WriteString(`", "class": "`)
	buf.WriteString(l.Class)
	l.serialize(buf, `", "`, `", "`, `": "`)
	buf.WriteString(`"}`)
}

func (l *Log) WriteTo(buf *bytes.Buffer) {
	buf.WriteString(l.date)
	buf.WriteByte(' ')
	buf.WriteString(l.time)
	buf.WriteByte('\t')
	buf.WriteString(l.Level)
	buf.WriteByte('\t')
	buf.WriteByte('[')
	if l.Thread != "" {
		buf.WriteString(l.Thread)
	} else {
		buf.WriteByte('-')
	}
	buf.WriteByte(']')
	buf.WriteByte('\t')
	if l.Class != "" {
		buf.WriteString(l.Class)
	} else {
		buf.WriteByte('-')
	}
	l.serialize(buf, "\t", ", ", ": ")
}

func (l *Log) String() string {
	var buf bytes.Buffer
	l.WriteTo(&buf)
	return buf.String()
}

// JSON serializes the log in JSON format
func (l *Log) JSON() string {
	var buf bytes.Buffer
	l.WriteJSONTo(&buf)
	return buf.String()
}

func (l *Log) Reader() io.Reader {
	var buf bytes.Buffer
	l.WriteTo(&buf)
	return &buf
}

func (l *Log) JSONReader() io.Reader {
	var buf bytes.Buffer
	l.WriteJSONTo(&buf)
	return &buf
}
