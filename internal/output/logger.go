// Package output provides formatted, color-coded logging for compose operations.
package output

import (
	"fmt"
	"io"
	"os"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

type Logger struct {
	stdout io.Writer
	stderr io.Writer
}

func NewLogger(stdout, stderr io.Writer) *Logger {
	return &Logger{stdout: stdout, stderr: stderr}
}

func (l *Logger) Stdout() io.Writer {
	return l.stdout
}

func (l *Logger) Stderr() io.Writer {
	return l.stderr
}

func (l *Logger) Infof(format string, args ...interface{}) {
	fmt.Fprintf(l.stderr, colorCyan+"[+] "+colorReset+format+"\n", args...)
}

func (l *Logger) Successf(format string, args ...interface{}) {
	fmt.Fprintf(l.stderr, colorGreen+" ✔ "+colorReset+format+"\n", args...)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	fmt.Fprintf(l.stderr, colorYellow+"[!] "+colorReset+format+"\n", args...)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(l.stderr, colorRed+"[✗] "+colorReset+format+"\n", args...)
}

// Debugf prints a debug message (only if COMPOSE_DEBUG is set).
func (l *Logger) Debugf(format string, args ...interface{}) {
	if os.Getenv("COMPOSE_DEBUG") != "" {
		fmt.Fprintf(l.stderr, colorGray+"[D] "+format+colorReset+"\n", args...)
	}
}
