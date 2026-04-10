package output

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestComposeProgressShowsAllServices(t *testing.T) {
	var buf bytes.Buffer
	services := [][2]string{
		{"web", "nginx:latest"},
		{"db", "postgres:16"},
		{"cache", "redis:7"},
	}
	cp := NewComposeProgress(&buf, services)
	time.Sleep(150 * time.Millisecond) // let initial render happen
	cp.Finish()

	out := buf.String()
	for _, s := range services {
		if !strings.Contains(out, s[0]) {
			t.Errorf("expected service name %q in output, got: %q", s[0], out)
		}
	}
}

func TestComposeProgressStateTransitions(t *testing.T) {
	var buf bytes.Buffer
	services := [][2]string{
		{"web", "nginx:latest"},
		{"db", "postgres:16"},
	}
	cp := NewComposeProgress(&buf, services)

	cp.SetState("web", StatePulling, nil)
	time.Sleep(150 * time.Millisecond)

	cp.SetState("web", StateDone, nil)
	cp.SetState("db", StateSkipped, nil)
	time.Sleep(150 * time.Millisecond)
	cp.Finish()

	out := buf.String()
	if !strings.Contains(out, "Pulled") {
		t.Errorf("expected 'Pulled' in output for completed service, got: %q", out)
	}
	if !strings.Contains(out, "Skipped") {
		t.Errorf("expected 'Skipped' in output for skipped service, got: %q", out)
	}
}

func TestComposeProgressErrorState(t *testing.T) {
	var buf bytes.Buffer
	services := [][2]string{
		{"web", "nginx:latest"},
	}
	cp := NewComposeProgress(&buf, services)

	cp.SetState("web", StatePulling, nil)
	time.Sleep(150 * time.Millisecond)

	cp.SetState("web", StateError, fmt.Errorf("connection refused"))
	time.Sleep(150 * time.Millisecond)
	cp.Finish()

	out := buf.String()
	if !strings.Contains(out, "connection refused") {
		t.Errorf("expected error message in output, got: %q", out)
	}
}

func TestServiceWriterUpdatesDetail(t *testing.T) {
	var buf bytes.Buffer
	services := [][2]string{
		{"web", "nginx:latest"},
	}
	cp := NewComposeProgress(&buf, services)

	cp.SetState("web", StatePulling, nil)
	w := cp.ServiceWriter("web")
	w.Write([]byte("Downloading layer 3/5\n"))
	time.Sleep(150 * time.Millisecond)
	cp.Finish()

	out := buf.String()
	if !strings.Contains(out, "Downloading layer 3/5") {
		t.Errorf("expected progress detail in output, got: %q", out)
	}
}

func TestServiceWriterStripsANSI(t *testing.T) {
	var buf bytes.Buffer
	services := [][2]string{
		{"web", "nginx:latest"},
	}
	cp := NewComposeProgress(&buf, services)

	cp.SetState("web", StatePulling, nil)
	w := cp.ServiceWriter("web")
	w.Write([]byte("\x1b[32mDownloading\x1b[0m\n"))
	time.Sleep(150 * time.Millisecond)
	cp.Finish()

	out := buf.String()
	if !strings.Contains(out, "Downloading") {
		t.Errorf("expected 'Downloading' in output, got: %q", out)
	}
}

func TestServiceWriterStripsSubprocessSpinner(t *testing.T) {
	var buf bytes.Buffer
	services := [][2]string{
		{"web", "nginx:latest"},
	}
	cp := NewComposeProgress(&buf, services)

	cp.SetState("web", StatePulling, nil)
	w := cp.ServiceWriter("web")
	// Simulate output from container CLI: "⠙ [1/6] Fetching image 10%"
	w.Write([]byte("⠙ [1/6] Fetching image 10%\r"))
	time.Sleep(150 * time.Millisecond)
	cp.Finish()

	out := buf.String()
	// The subprocess spinner should be stripped.
	if !strings.Contains(out, "[1/6] Fetching image 10%") {
		t.Errorf("expected cleaned progress in output, got: %q", out)
	}
}

func TestServiceWriterParsesPercentage(t *testing.T) {
	var buf bytes.Buffer
	services := [][2]string{
		{"web", "nginx:latest"},
	}
	cp := NewComposeProgress(&buf, services)

	cp.SetState("web", StatePulling, nil)
	w := cp.ServiceWriter("web")
	w.Write([]byte("[1/6] Fetching image 45% (9 of 18 blobs)\r"))
	time.Sleep(150 * time.Millisecond)

	cp.mu.Lock()
	pct := cp.services[0].percent
	cp.mu.Unlock()

	cp.Finish()

	if pct != 45 {
		t.Errorf("expected parsed percentage 45, got: %d", pct)
	}

	out := buf.String()
	// Should contain the progress bar with 45%.
	if !strings.Contains(out, "45%") {
		t.Errorf("expected '45%%' in output, got: %q", out)
	}
}

func TestProgressBarRendering(t *testing.T) {
	tests := []struct {
		percent int
		wantPct string
	}{
		{0, "0%"},
		{50, "50%"},
		{100, "100%"},
	}
	for _, tc := range tests {
		bar := progressBar(tc.percent)
		// Strip ANSI to check content.
		clean := ansiPattern.ReplaceAllString(bar, "")
		if !strings.Contains(clean, tc.wantPct) {
			t.Errorf("progressBar(%d): expected %q in output, got: %q", tc.percent, tc.wantPct, clean)
		}
		// Check bar characters are present.
		if !strings.Contains(clean, "[") || !strings.Contains(clean, "]") {
			t.Errorf("progressBar(%d): expected bracket delimiters, got: %q", tc.percent, clean)
		}
	}
}

func TestComposeProgressHidesCursor(t *testing.T) {
	var buf bytes.Buffer
	services := [][2]string{
		{"web", "nginx:latest"},
	}
	cp := NewComposeProgress(&buf, services)
	time.Sleep(150 * time.Millisecond)
	cp.Finish()

	out := buf.String()
	// Should contain cursor hide at start and cursor show at end.
	if !strings.Contains(out, "\033[?25l") {
		t.Error("expected cursor hide escape sequence in output")
	}
	if !strings.Contains(out, "\033[?25h") {
		t.Error("expected cursor show escape sequence in output")
	}
}

func TestComposeProgressUnknownService(t *testing.T) {
	var buf bytes.Buffer
	services := [][2]string{
		{"web", "nginx:latest"},
	}
	cp := NewComposeProgress(&buf, services)

	// Should not panic for unknown service names.
	cp.SetState("nonexistent", StatePulling, nil)
	w := cp.ServiceWriter("nonexistent")
	w.Write([]byte("should not crash\n"))
	cp.Finish()
}
