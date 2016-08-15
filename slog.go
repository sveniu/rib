package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// A SimpleLogger represents a logging object that generates lines of output to
// an io.Writer. It includes a debug flag to control output of debug messages.
type SimpleLogger struct {
	out   io.Writer
	debug bool
}

// EnableDebug enables the debug flag on the logger.
func (l *SimpleLogger) EnableDebug() {
	l.debug = true
}

// NewLogger creates a new SimpleLogger.
func NewLogger(out io.Writer, prefix string, flag int) *SimpleLogger {
	return &SimpleLogger{out: out, debug: false}
}

var Std = NewLogger(os.Stdout, "", 0)

// Output writes the output for a logging event. It is a simple adaption of
// https://golang.org/pkg/log/#Output
func (l *SimpleLogger) Output(calldepth int, s string) error {
	now := time.Now()

	if s[len(s)-1] == '\n' {
		s = s[0 : len(s)-1]
	}
	qs := strings.Trim(strconv.QuoteToASCII(string(s)), `"`)

	var os string
	if l.debug {
		var fname string
		pc, file, line, ok := runtime.Caller(calldepth)
		if ok {
			file = filepath.Base(file)
			fname = runtime.FuncForPC(pc).Name()
		} else {
			file = "???"
			line = 0
			fname = "???"
		}
		os = fmt.Sprintf("%s %s:%s():%d %s\n",
			now.UTC().Format("2006-01-02T15:04:05.000Z"),
			file, fname, line,
			qs,
		)
	} else {
		os = fmt.Sprintf("%s %s\n",
			now.UTC().Format("2006-01-02T15:04:05.000Z"),
			qs,
		)
	}

	_, err := l.out.Write([]byte(os))
	return err
}

// SetStandard sets the standard logger to be itself.
func (l *SimpleLogger) SetStandard() {
	Std = l
}

// SetOutput sets the given writer as the output.
func (l *SimpleLogger) SetOutput(w io.Writer) {
	l.out = w
}

// AddOutput adds the given writer to the logger output.
func (l *SimpleLogger) AddOutput(w io.Writer) {
	l.out = io.MultiWriter(l.out, w)
}

// AddLoggerOutput adds the given writer to the standard logger output.
func AddLoggerOutput(w io.Writer) {
	Std.out = io.MultiWriter(Std.out, w)
}

// Debugf calls Output to print to the standard logger with a "DEBUG" prefix.
func Debugf(format string, v ...interface{}) {
	Std.Output(2, fmt.Sprintf("DEBUG "+format, v...))
}

// Debugf calls Output to print to the standard logger with a "INFO" prefix.
func Infof(format string, v ...interface{}) {
	Std.Output(2, fmt.Sprintf("INFO "+format, v...))
}

// Debugf calls Output to print to the standard logger with a "WARNING" prefix.
func Warningf(format string, v ...interface{}) {
	Std.Output(2, fmt.Sprintf("WARNING "+format, v...))
}

// Debugf calls Output to print to the standard logger with a "ERROR" prefix.
func Errorf(format string, v ...interface{}) {
	Std.Output(2, fmt.Sprintf("ERROR "+format, v...))
}
