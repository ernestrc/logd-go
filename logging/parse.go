package logging

type state uint8

const (
	dateState state = iota
	timeState
	transitionLevelState
	levelState
	transitionThreadState
	threadState
	transitionClassState
	classState
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
	//	p.consumeCurrent(log)
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

func (p *Parser) consumeCurrent(log *Log) {
	i := len(log.props) - 1
	if i < 0 {
		return
	}
	currProp := log.props[i]
	var field *string
	switch currProp.key {
	case "step":
		field = &log.step
	case "flow":
		field = &log.flow
	case "operation":
		field = &log.operation
	case "traceId":
		field = &log.traceID
	default:
		log.props[i] = Property{currProp.key, p.raw[p.start:p.end]}
		return
	}

	*field = p.raw[p.start:p.end]
	log.props = log.props[:i]
}

func (p *Parser) handleNextValue(log *Log, r rune) {
	switch r {
	case ',':
		p.consumeCurrent(log)
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
	case '\t':
		fallthrough
	case ' ':
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
	case '\t':
		fallthrough
	case ' ':
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
		p.end++
		log.thread = p.raw[p.start:p.end]
		p.start = p.end
		p.state++
	default:
		p.end++
	}
}

func (p *Parser) next(log *Log, r rune) {
	switch p.state {
	case dateState:
		p.handleNextHeader(&log.date, r)
	case timeState:
		p.handleNextHeader(&log.time, r)
	case levelState:
		p.handleNextHeader(&log.level, r)
	case threadState:
		p.handleNextThread(log, r)
	case classState:
		p.handleNextHeader(&log.class, r)
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
			p.consumeCurrent(&p.current)
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
