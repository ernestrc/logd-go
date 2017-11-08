package main

import (
	"bufio"
	"io"
)

var newLine = []byte{'\n'}

// LogWriter is a buffered stdout Writer that writes data in chunks
// separated by newlines
type LogWriter struct {
	writer *bufio.Writer
}

func NewLogWriter(writer io.Writer) *LogWriter {
	w := new(LogWriter)
	w.writer = bufio.NewWriter(writer)
	return w
}

func (w *LogWriter) Write(b []byte) (n int, err error) {
	n, err = w.writer.Write(b)
	if err != nil {
		return
	}
	_, err = w.writer.Write(newLine)
	n++
	return
}

func (w *LogWriter) Flush() error {
	return w.writer.Flush()
}
