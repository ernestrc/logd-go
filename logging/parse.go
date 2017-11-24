package logging

type state uint8

const (
	// TODO should ignore any rune that is ^(1-9|:|,|.)
	dateState state = iota
	timeState
	transitionLevelState
	levelState
	transitionThreadState
	threadState
	transitionClassState
	classState
	transitionCallTypeState
	callTypeState
	verifyCallTypeState
	keyState
	multiKeyState
	valueState
	errorState
)

// Parser holds the state of input text parsing
type Parser struct {
	state
	start, end int
	raw        string
	current    Log
}

func (p *Parser) handleNextKey(log *Log, r rune) {
	// TODO handle , and push to prev Property
	switch r {
	case ' ':
		// trim left spaces
		if p.start == p.end {
			p.start++
			p.end++
		}
	case ':':
		log.props = append(log.props, Property{key: p.raw[p.start:p.end]})
		p.state = valueState
		p.end++
		p.start = p.end
	default:
		p.end++
	}
}

func (p *Parser) handleNextMultiKey(log *Log, r rune) {
	switch r {
	// TODO case ',':
	//	p.consumeCurrent()
	//	p.state = keyState
	//	p.end++
	//	p.start = p.end
	case ' ':
		// clean first key
		log.props[len(log.props)-1] = Property{key: p.raw[p.start : p.end-1]}
		p.end++
		p.start = p.end
		p.state = valueState
	default:
		// reset state as char:char is legitimate
		p.state = valueState
		p.end++
	}
}

func (p *Parser) consumeCurrent() {
	log := &p.current
	i := len(log.props) - 1
	if i < 0 {
		return
	}
	log.props[i].value = p.raw[p.start:p.end]
}

func (p *Parser) consumeLog() {
	log := &p.current
	i := len(log.props) - 1
	if i < 0 || log.props[i].value != "" {
		// key has not been consumed yet so remainging is message
		log.Message = p.raw[p.start:p.end]
		return
	}

	// key has been consumed, so remainging is value
	log.props[i].value = p.raw[p.start:p.end]
}

func (p *Parser) handleNextValue(log *Log, r rune) {
	switch r {
	case ',':
		p.consumeCurrent()
		p.state = keyState
		p.end++
		p.start = p.end
	case ':': // ":" followed by a space is a double key and will be ignored
		p.state = multiKeyState
		p.end++
	case ' ':
		// trim left spaces
		if p.start == p.end {
			p.start++
		}
		p.end++
	default:
		p.end++
	}
}

func (p *Parser) handleTransition(r rune) bool {
	switch r {
	case '\t', ' ':
		p.start++
		p.end++
	default:
		p.state++
		return true
	}

	return false
}

func (p *Parser) handleNextHeader(prop *string, r rune) {
	switch r {
	case '\t', ' ':
		*prop = p.raw[p.start:p.end]
		p.end++
		p.start = p.end
		p.state++
	default:
		p.end++
	}
}

func (p *Parser) handleNextThread(log *Log, r rune) {
	switch r {
	case ']':
		p.start++
		log.Thread = p.raw[p.start:p.end]
		p.end++
		p.start = p.end
		p.state++
	default:
		p.end++
	}
}

func (p *Parser) handleNextCallType(log *Log, r rune) {
	switch r {
	case ':':
		log.props = append(log.props, Property{key: "callType"})
		p.consumeCurrent()
		p.end++
		p.start = p.end
		p.state++
	default:
		p.end++
	}
}

func (p *Parser) verifyCallType(log *Log, r rune) {
	switch r {
	// was not callType
	case ',':
		log.props[0].key = log.props[0].value
		p.handleNextValue(log, r)
	default:
		p.handleNextKey(log, r)
	}
}

func (p *Parser) next(log *Log, r rune) {
	switch p.state {
	case dateState:
		p.handleNextHeader(&log.date, r)
	case timeState:
		p.handleNextHeader(&log.time, r)
	case levelState:
		p.handleNextHeader(&log.Level, r)
	case threadState:
		p.handleNextThread(log, r)
	case classState:
		p.handleNextHeader(&log.Class, r)
	case callTypeState:
		p.handleNextCallType(log, r)
	// callType is only committed if there are two keys in sequence: "callType: key: value"
	case verifyCallTypeState:
		p.verifyCallType(log, r)
	case keyState:
		p.handleNextKey(log, r)
	case valueState:
		p.handleNextValue(log, r)
	case multiKeyState:
		p.handleNextMultiKey(log, r)
	case errorState:
		// ignore until newline is found and state is reset
	default:
		if p.handleTransition(r) {
			p.next(log, r)
		}
	}
}

// Parse will append the logs parsed in chunk in logs slice and return the slice
func (p *Parser) Parse(chunk string, logs []Log) []Log {
	p.raw += chunk

	for i, r := range p.raw {
		switch r {
		case '\n':
			p.consumeLog()
			p.start = i + 1
			p.end = p.start
			p.state = dateState
			logs = append(logs, p.current)
			p.current = Log{props: make([]Property, 0)}
		default:
			p.next(&p.current, r)
		}
	}

	p.raw = p.raw[p.start:]
	p.start, p.end = 0, 0

	return logs
}

// NewParser allocates storage for a Parser and initializes it with the given string
func NewParser() (p *Parser) {
	p = new(Parser)
	p.Reset()
	return
}

// Reset resets the Parser to use the given chunk
func (p *Parser) Reset() {
	p.state = dateState
	p.start, p.end = 0, 0
}

// Parse parses raw text into structured logs
func Parse(raw string) (logs []Log) {
	p := NewParser()
	logs = p.Parse(raw, make([]Log, 0))
	return
}
