package output

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	var stdout, stderr bytes.Buffer
	logger := NewLogger(&stdout, &stderr)

	if logger.Stdout() != &stdout {
		t.Error("Stdout() returned wrong writer")
	}
	if logger.Stderr() != &stderr {
		t.Error("Stderr() returned wrong writer")
	}
}

func TestInfof(t *testing.T) {
	var stderr bytes.Buffer
	logger := NewLogger(nil, &stderr)

	logger.Infof("hello %s", "world")
	out := stderr.String()

	if !strings.Contains(out, "hello world") {
		t.Errorf("expected 'hello world' in output, got: %q", out)
	}
	if !strings.Contains(out, "[+]") {
		t.Errorf("expected '[+]' prefix in output, got: %q", out)
	}
}

func TestSuccessf(t *testing.T) {
	var stderr bytes.Buffer
	logger := NewLogger(nil, &stderr)

	logger.Successf("done %d items", 5)
	out := stderr.String()

	if !strings.Contains(out, "done 5 items") {
		t.Errorf("expected 'done 5 items' in output, got: %q", out)
	}
	if !strings.Contains(out, "✔") {
		t.Errorf("expected '✔' prefix in output, got: %q", out)
	}
}

func TestWarnf(t *testing.T) {
	var stderr bytes.Buffer
	logger := NewLogger(nil, &stderr)

	logger.Warnf("caution: %s", "slow")
	out := stderr.String()

	if !strings.Contains(out, "caution: slow") {
		t.Errorf("expected 'caution: slow' in output, got: %q", out)
	}
	if !strings.Contains(out, "[!]") {
		t.Errorf("expected '[!]' prefix in output, got: %q", out)
	}
}

func TestErrorf(t *testing.T) {
	var stderr bytes.Buffer
	logger := NewLogger(nil, &stderr)

	logger.Errorf("failed: %v", "timeout")
	out := stderr.String()

	if !strings.Contains(out, "failed: timeout") {
		t.Errorf("expected 'failed: timeout' in output, got: %q", out)
	}
	if !strings.Contains(out, "✗") {
		t.Errorf("expected '✗' prefix in output, got: %q", out)
	}
}

func TestDebugfDisabled(t *testing.T) {
	os.Unsetenv("COMPOSE_DEBUG")
	var stderr bytes.Buffer
	logger := NewLogger(nil, &stderr)

	logger.Debugf("should not appear")
	if stderr.Len() != 0 {
		t.Errorf("expected no output without COMPOSE_DEBUG, got: %q", stderr.String())
	}
}

func TestDebugfEnabled(t *testing.T) {
	os.Setenv("COMPOSE_DEBUG", "1")
	defer os.Unsetenv("COMPOSE_DEBUG")

	var stderr bytes.Buffer
	logger := NewLogger(nil, &stderr)

	logger.Debugf("debug %s", "info")
	out := stderr.String()

	if !strings.Contains(out, "debug info") {
		t.Errorf("expected 'debug info' in output, got: %q", out)
	}
	if !strings.Contains(out, "[D]") {
		t.Errorf("expected '[D]' prefix in output, got: %q", out)
	}
}
