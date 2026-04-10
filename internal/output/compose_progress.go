package output

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ServiceState represents the current pull state of a service.
type ServiceState int

const (
	StateWaiting ServiceState = iota
	StatePulling
	StateDone
	StateError
	StateSkipped
)

// serviceEntry tracks the display state for one service in the compose file.
type serviceEntry struct {
	name    string
	image   string
	state   ServiceState
	detail  string // cleaned progress detail from subprocess
	percent int    // parsed percentage (0-100), -1 if unknown
	err     error
}

// spinnerFrames provides a smooth animation for active operations.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// progressBarWidth is the number of characters in the visual progress bar.
const progressBarWidth = 20

// subprocessSpinner strips leading Braille spinner characters from the
// container CLI output (e.g. "⠙ [1/6] ...").
var subprocessSpinner = regexp.MustCompile(`^[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏⠿⠾⠽⠻⠟⠯⠷⠶⠵⠳⠺⠹]+\s*`)

// percentPattern extracts a percentage like "10%" from progress output.
var percentPattern = regexp.MustCompile(`(\d{1,3})%`)

// ComposeProgress renders a flicker-free, multi-line progress display
// for pulling all images in a compose file. Each service occupies one
// line that is updated in-place using ANSI cursor movement.
//
// It hides the terminal cursor during rendering and restores it on
// Finish() to eliminate visible cursor flicker.
type ComposeProgress struct {
	mu        sync.Mutex
	out       io.Writer
	services  []*serviceEntry
	index     map[string]int // service name -> index in services slice
	rendered  int            // number of lines we have rendered so far
	frame     int            // spinner animation frame counter
	stop      chan struct{}
	done      chan struct{}
	maxName   int    // longest service name (for alignment)
	doneLabel string // label shown for completed services (default "Pulled")
}

// NewComposeProgress creates a progress display for the given list of
// service/image pairs. Call Finish() when all pulls are complete.
// services is a slice of [2]string{serviceName, imageName}.
func NewComposeProgress(out io.Writer, services [][2]string) *ComposeProgress {
	cp := &ComposeProgress{
		out:       out,
		index:     make(map[string]int, len(services)),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		doneLabel: "Pulled",
	}
	for i, s := range services {
		cp.services = append(cp.services, &serviceEntry{
			name:    s[0],
			image:   s[1],
			state:   StateWaiting,
			percent: -1,
		})
		cp.index[s[0]] = i
		if len(s[0]) > cp.maxName {
			cp.maxName = len(s[0])
		}
	}
	// Hide cursor to prevent flicker.
	fmt.Fprint(out, "\033[?25l")
	go cp.loop()
	return cp
}

// SetDoneLabel sets the label shown for completed services (default "Pulled").
// Use "Started" for the up command, "Pulled" for pull, etc.
func (cp *ComposeProgress) SetDoneLabel(label string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.doneLabel = label
}

func (cp *ComposeProgress) loop() {
	defer close(cp.done)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cp.frame++
			cp.render()
		case <-cp.stop:
			cp.render() // final render
			return
		}
	}
}

// ServiceWriter returns an io.Writer that captures subprocess output
// for the named service and feeds it into the progress display.
func (cp *ComposeProgress) ServiceWriter(service string) io.Writer {
	return &serviceProgressWriter{cp: cp, service: service}
}

// SetState updates the state for a service (e.g. pulling, done, error).
func (cp *ComposeProgress) SetState(service string, state ServiceState, err error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	idx, ok := cp.index[service]
	if !ok {
		return
	}
	cp.services[idx].state = state
	if err != nil {
		cp.services[idx].err = err
	}
}

// Finish stops the render loop, does a final render, and restores
// the cursor so subsequent output works normally.
func (cp *ComposeProgress) Finish() {
	close(cp.stop)
	<-cp.done // wait for render goroutine to finish
	// Show cursor again.
	fmt.Fprint(cp.out, "\033[?25h")
}

func (cp *ComposeProgress) render() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Move cursor up to overwrite previous render.
	if cp.rendered > 0 {
		fmt.Fprintf(cp.out, "\033[%dA", cp.rendered)
	}

	lines := 0
	for _, svc := range cp.services {
		line := cp.formatLine(svc)
		// Move to column 1, erase the entire line, write new content.
		fmt.Fprintf(cp.out, "\r\033[2K%s\n", line)
		lines++
	}
	cp.rendered = lines
}

// progressBar renders a visual bar like [████████░░░░░░░░░░░░] 45%.
func progressBar(percent int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := percent * progressBarWidth / 100
	empty := progressBarWidth - filled
	return fmt.Sprintf("%s[%s%s%s%s]%s %d%%",
		colorCyan,
		colorGreen,
		strings.Repeat("█", filled),
		colorGray,
		strings.Repeat("░", empty),
		colorReset,
		percent,
	)
}

func (cp *ComposeProgress) formatLine(svc *serviceEntry) string {
	padded := svc.name + strings.Repeat(" ", cp.maxName-len(svc.name))

	switch svc.state {
	case StateWaiting:
		return fmt.Sprintf(" %s·%s %s %sWaiting%s",
			colorGray, colorReset,
			padded,
			colorGray, colorReset)

	case StatePulling:
		spinner := spinnerFrames[cp.frame%len(spinnerFrames)]

		if svc.percent >= 0 {
			// Show progress bar with detail.
			bar := progressBar(svc.percent)
			return fmt.Sprintf(" %s%s%s %s %s %s%s%s",
				colorCyan, spinner, colorReset,
				padded,
				bar,
				colorGray, svc.detail, colorReset)
		}

		// No percentage parsed yet -- show detail or "Pulling <image>".
		detail := svc.detail
		if detail == "" {
			detail = "Pulling " + svc.image
		}
		return fmt.Sprintf(" %s%s%s %s %s%s%s",
			colorCyan, spinner, colorReset,
			padded,
			colorGray, detail, colorReset)

	case StateDone:
		label := cp.doneLabel
		if label == "" {
			label = "Pulled"
		}
		return fmt.Sprintf(" %s✔%s %s %s%s%s",
			colorGreen, colorReset,
			padded,
			colorGreen, label, colorReset)

	case StateError:
		errMsg := "failed"
		if svc.err != nil {
			errMsg = svc.err.Error()
		}
		return fmt.Sprintf(" %s✗%s %s %s%s%s",
			colorRed, colorReset,
			padded,
			colorRed, errMsg, colorReset)

	case StateSkipped:
		return fmt.Sprintf(" %s-%s %s %sSkipped (no image)%s",
			colorGray, colorReset,
			padded,
			colorGray, colorReset)

	default:
		return fmt.Sprintf("   %s %s", padded, svc.detail)
	}
}

// serviceProgressWriter is an io.Writer adapter that captures subprocess
// output for a single service and updates the ComposeProgress display.
type serviceProgressWriter struct {
	cp      *ComposeProgress
	service string
}

func (w *serviceProgressWriter) Write(p []byte) (int, error) {
	// Strip ANSI escape codes.
	clean := ansiPattern.ReplaceAllString(string(p), "")

	var lastLine string
	for _, seg := range strings.FieldsFunc(clean, func(r rune) bool {
		return r == '\n' || r == '\r'
	}) {
		if t := strings.TrimSpace(seg); t != "" {
			lastLine = t
		}
	}
	if lastLine == "" {
		return len(p), nil
	}

	// Strip the subprocess's own Braille spinner prefix.
	lastLine = subprocessSpinner.ReplaceAllString(lastLine, "")
	lastLine = strings.TrimSpace(lastLine)
	if lastLine == "" {
		return len(p), nil
	}

	// Try to extract a percentage.
	pct := -1
	if m := percentPattern.FindStringSubmatch(lastLine); m != nil {
		fmt.Sscanf(m[1], "%d", &pct)
	}

	w.cp.mu.Lock()
	if idx, ok := w.cp.index[w.service]; ok {
		w.cp.services[idx].detail = lastLine
		if pct >= 0 {
			w.cp.services[idx].percent = pct
		}
	}
	w.cp.mu.Unlock()

	return len(p), nil
}
