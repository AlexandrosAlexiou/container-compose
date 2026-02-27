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

// Logger provides formatted output for compose operations.
type Logger struct {
	stdout io.Writer
	stderr io.Writer
}

// NewLogger creates a new Logger.
func NewLogger(stdout, stderr io.Writer) *Logger {
	return &Logger{stdout: stdout, stderr: stderr}
}

// Stdout returns the stdout writer.
func (l *Logger) Stdout() io.Writer {
	return l.stdout
}

// Stderr returns the stderr writer.
func (l *Logger) Stderr() io.Writer {
	return l.stderr
}

// Infof prints an informational message.
func (l *Logger) Infof(format string, args ...interface{}) {
	fmt.Fprintf(l.stderr, colorCyan+"[+] "+colorReset+format+"\n", args...)
}

// Successf prints a success message.
func (l *Logger) Successf(format string, args ...interface{}) {
	fmt.Fprintf(l.stderr, colorGreen+" ✔ "+colorReset+format+"\n", args...)
}

// Warnf prints a warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	fmt.Fprintf(l.stderr, colorYellow+"[!] "+colorReset+format+"\n", args...)
}

// Errorf prints an error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(l.stderr, colorRed+"[✗] "+colorReset+format+"\n", args...)
}

// Debugf prints a debug message (only if COMPOSE_DEBUG is set).
func (l *Logger) Debugf(format string, args ...interface{}) {
	if os.Getenv("COMPOSE_DEBUG") != "" {
		fmt.Fprintf(l.stderr, colorGray+"[D] "+format+colorReset+"\n", args...)
	}
}
