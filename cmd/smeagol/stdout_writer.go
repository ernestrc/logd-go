package main

import (
	"bufio"
	"io"
	"os"
)

var stdoutLogWriter = NewStdoutLogWriter()
var newLine = []byte{'\n'}

// StdoutLogWriter is a buffered stdout Writer that writes data in chunks
// separated by newlines
type StdoutLogWriter struct {
	writer io.Writer
}

func NewStdoutLogWriter() *StdoutLogWriter {
	w := new(StdoutLogWriter)
	w.writer = bufio.NewWriter(os.Stdout)
	return w
}

func (w *StdoutLogWriter) Write(b []byte) (n int, err error) {
	n, err = w.writer.Write(b)
	if err != nil {
		return
	}
	_, err = w.writer.Write(newLine)
	n++
	return
}
