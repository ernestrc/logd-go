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

type parserState struct {
	state
	start, end int
	raw        string
}

func (p *parserState) handleNextKey(log *Log, r rune) {
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

func (p *parserState) handleNextMultiKey(log *Log, r rune) {
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

func (p *parserState) consumeCurrent(log *Log) {
	i := len(log.props) - 1
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

func (p *parserState) handleNextValue(log *Log, r rune) {
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

func (p *parserState) handleTransition(r rune) bool {
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

func (p *parserState) handleNextHeader(prop *string, r rune) {
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

func (p *parserState) handleNextThread(log *Log, r rune) {
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

func (p *parserState) next(log *Log, r rune) {
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
