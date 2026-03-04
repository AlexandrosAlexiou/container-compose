package output

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestProgressWriterStripsANSI(t *testing.T) {
	var buf bytes.Buffer
	pw := NewProgressWriter(&buf)

	pw.Write([]byte("\x1b[32mDownloading layer\x1b[0m\n"))
	time.Sleep(200 * time.Millisecond)
	pw.Finish()

	out := buf.String()
	if strings.Contains(out, "\x1b[") {
		t.Errorf("output should not contain ANSI codes, got: %q", out)
	}
}

func TestProgressWriterShowsLastLine(t *testing.T) {
	var buf bytes.Buffer
	pw := NewProgressWriter(&buf)

	pw.Write([]byte("line one\nline two\nline three\n"))
	time.Sleep(200 * time.Millisecond)
	pw.Finish()

	out := buf.String()
	if !strings.Contains(out, "line three") {
		t.Errorf("expected 'line three' in output, got: %q", out)
	}
}

func TestProgressWriterHandlesCR(t *testing.T) {
	var buf bytes.Buffer
	pw := NewProgressWriter(&buf)

	pw.Write([]byte("progress 10%\rprogress 50%\rprogress 100%"))
	time.Sleep(200 * time.Millisecond)
	pw.Finish()

	out := buf.String()
	if !strings.Contains(out, "progress 100%") {
		t.Errorf("expected 'progress 100%%' in output, got: %q", out)
	}
}

func TestProgressWriterClearsLine(t *testing.T) {
	var buf bytes.Buffer
	pw := NewProgressWriter(&buf)

	pw.Write([]byte("downloading...\n"))
	time.Sleep(200 * time.Millisecond)
	pw.Finish()

	out := buf.String()
	// After Finish(), the line should end with \r (cleared)
	if !strings.HasSuffix(out, "\r") {
		t.Errorf("expected output to end with carriage return after Finish, got: %q", out)
	}
}
