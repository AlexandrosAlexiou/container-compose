package output

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[?]?[0-9;]*[a-zA-Z]`)

// ProgressWriter captures raw subprocess output and renders a clean,
// throttled single-line progress indicator to avoid terminal flickering.
type ProgressWriter struct {
	mu       sync.Mutex
	out      io.Writer
	lastLine string
	prevLen  int
	dirty    bool
	stop     chan struct{}
}

// NewProgressWriter creates a writer that throttles output to at most
// one update every 150ms, collapsing multi-line progress into a single
// overwritten line.
func NewProgressWriter(out io.Writer) *ProgressWriter {
	pw := &ProgressWriter{
		out:  out,
		stop: make(chan struct{}),
	}
	go pw.loop()
	return pw
}

func (pw *ProgressWriter) loop() {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			pw.render()
		case <-pw.stop:
			return
		}
	}
}

// Write implements io.Writer. It strips ANSI escape codes and extracts the
// latest meaningful line from the subprocess output.
func (pw *ProgressWriter) Write(p []byte) (int, error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	clean := ansiPattern.ReplaceAllString(string(p), "")
	for _, seg := range strings.FieldsFunc(clean, func(r rune) bool {
		return r == '\n' || r == '\r'
	}) {
		if t := strings.TrimSpace(seg); t != "" {
			pw.lastLine = t
			pw.dirty = true
		}
	}
	return len(p), nil
}

func (pw *ProgressWriter) render() {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	if !pw.dirty {
		return
	}
	pw.dirty = false

	line := pw.lastLine

	pad := ""
	if diff := pw.prevLen - len(line); diff > 0 {
		pad = strings.Repeat(" ", diff)
	}
	pw.prevLen = len(line)

	fmt.Fprintf(pw.out, "\r    %s%s", line, pad)
}

// Finish stops the render loop, flushes the last state, and clears the
// progress line so subsequent output starts on a clean line.
func (pw *ProgressWriter) Finish() {
	close(pw.stop)
	pw.render()
	pw.mu.Lock()
	defer pw.mu.Unlock()
	if pw.prevLen > 0 {
		fmt.Fprintf(pw.out, "\r%s\r", strings.Repeat(" ", pw.prevLen+4))
	}
}
