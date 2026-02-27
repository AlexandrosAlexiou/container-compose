//go:build integration

// Package integration provides end-to-end tests that require a running
// Apple Container runtime. Run with:
//
//	go test -tags integration -v -timeout 600s ./internal/integration/
package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// binary returns the path to the container-compose binary.
// It builds the binary if it doesn't exist.
func binary(t *testing.T) string {
	t.Helper()

	// Find the project root (two levels up from internal/integration/)
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")

	bin := filepath.Join(root, "bin", "container-compose")
	if _, err := os.Stat(bin); err != nil {
		t.Logf("Building container-compose binary...")
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/container-compose")
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to build binary: %v\n%s", err, out)
		}
	}
	return bin
}

// fixtureDir returns the absolute path to a test fixture directory.
func fixtureDir(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "fixtures", name)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("Fixture %q not found at %s", name, dir)
	}
	return dir
}

// compose runs container-compose with the given fixture and arguments.
func compose(t *testing.T, fixture string, args ...string) (string, error) {
	t.Helper()
	bin := binary(t)
	dir := fixtureDir(t, fixture)

	fullArgs := append([]string{"--project-directory", dir}, args...)
	t.Logf("Running: %s %s", bin, strings.Join(fullArgs, " "))

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, fullArgs...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// composeUp starts a fixture stack in detached mode.
func composeUp(t *testing.T, fixture string) {
	t.Helper()
	out, err := compose(t, fixture, "up", "-d")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, out)
	}
	t.Log(out)
}

// composeDown tears down a fixture stack and removes volumes.
func composeDown(t *testing.T, fixture string) {
	t.Helper()
	out, err := compose(t, fixture, "down", "-v")
	if err != nil {
		t.Logf("down warning: %v\n%s", err, out)
	}
	t.Log(out)
}

// containerExec runs a command inside a container and returns stdout.
func containerExec(t *testing.T, containerName string, cmd ...string) (string, error) {
	t.Helper()
	args := append([]string{"exec", containerName}, cmd...)
	c := exec.Command("container", args...)
	out, err := c.CombinedOutput()
	return string(out), err
}

// requireContainerRuntime skips the test if the container runtime isn't available.
func requireContainerRuntime(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("container"); err != nil {
		t.Skip("Skipping: 'container' CLI not found in PATH")
	}
	cmd := exec.Command("container", "list", "--format", "json")
	if err := cmd.Run(); err != nil {
		t.Skip("Skipping: container runtime not running (try 'brew services start container')")
	}
}

// --- Tests ---

func TestSimple(t *testing.T) {
	requireContainerRuntime(t)
	fixture := "simple"
	defer composeDown(t, fixture)

	out, err := compose(t, fixture, "up", "-d")
	if err != nil {
		t.Fatalf("up failed: %v\n%s", err, out)
	}

	// ps should list the container
	out, err = compose(t, fixture, "ps")
	if err != nil {
		t.Fatalf("ps failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("ps output should mention 'hello' service, got:\n%s", out)
	}
}

func TestMultiService(t *testing.T) {
	requireContainerRuntime(t)
	fixture := "multi-service"
	defer composeDown(t, fixture)

	composeUp(t, fixture)

	// Both services should be running
	out, _ := compose(t, fixture, "ps")
	if !strings.Contains(out, "backend") {
		t.Error("backend service not found in ps output")
	}
	if !strings.Contains(out, "frontend") {
		t.Error("frontend service not found in ps output")
	}

	// Verify both containers are running
	for _, svc := range []string{"backend", "frontend"} {
		containerName := fmt.Sprintf("multi-service-%s-1", svc)
		out, err := containerExec(t, containerName, "echo", "alive")
		if err != nil {
			t.Errorf("exec in %s failed: %v\n%s", svc, err, out)
		}
		if !strings.Contains(out, "alive") {
			t.Errorf("expected 'alive' from %s exec, got: %s", svc, out)
		}
	}
}

func TestEnvironment(t *testing.T) {
	requireContainerRuntime(t)
	fixture := "environment"
	defer composeDown(t, fixture)

	composeUp(t, fixture)

	containerName := "environment-web-1"

	// Verify environment variables are set
	tests := map[string]string{
		"APP_ENV":    "production",
		"APP_DEBUG":  "false",
		"SECRET_KEY": "s3cr3t",
	}
	for key, expected := range tests {
		out, err := containerExec(t, containerName, "printenv", key)
		if err != nil {
			t.Errorf("printenv %s failed: %v\n%s", key, err, out)
			continue
		}
		got := strings.TrimSpace(out)
		if got != expected {
			t.Errorf("env %s = %q, want %q", key, got, expected)
		}
	}
}

func TestVolumes(t *testing.T) {
	requireContainerRuntime(t)
	fixture := "volumes"
	defer composeDown(t, fixture)

	composeUp(t, fixture)

	containerName := "volumes-app-1"

	// Write a file to the volume
	_, err := containerExec(t, containerName, "sh", "-c", "echo testdata > /data/test.txt")
	if err != nil {
		t.Fatalf("write to volume failed: %v", err)
	}

	// Read it back
	out, err := containerExec(t, containerName, "cat", "/data/test.txt")
	if err != nil {
		t.Fatalf("read from volume failed: %v", err)
	}
	if !strings.Contains(out, "testdata") {
		t.Errorf("volume data mismatch: got %q", out)
	}
}

func TestServiceDiscovery(t *testing.T) {
	requireContainerRuntime(t)
	fixture := "service-discovery"
	defer composeDown(t, fixture)

	composeUp(t, fixture)

	clientContainer := "service-discovery-client-1"

	// Verify /etc/hosts has the server entry
	out, err := containerExec(t, clientContainer, "cat", "/etc/hosts")
	if err != nil {
		t.Fatalf("cat /etc/hosts failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "server") {
		t.Errorf("/etc/hosts should contain 'server' entry, got:\n%s", out)
	}

	// Verify DNS resolution of service name
	out, err = containerExec(t, clientContainer, "getent", "hosts", "server")
	if err != nil {
		t.Fatalf("getent hosts server failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "server") {
		t.Errorf("getent should resolve 'server', got: %s", out)
	}

	// Verify ping to server by service name
	out, err = containerExec(t, clientContainer, "ping", "-c", "1", "-W", "3", "server")
	if err != nil {
		t.Errorf("ping server failed: %v\n%s", err, out)
	}
}

func TestNetworks(t *testing.T) {
	requireContainerRuntime(t)
	fixture := "networks"
	defer composeDown(t, fixture)

	composeUp(t, fixture)

	// Verify container is on the custom network
	out, _ := compose(t, fixture, "ps")
	if !strings.Contains(out, "app") {
		t.Error("app service not found in ps output")
	}

	containerName := "networks-app-1"
	out, err := containerExec(t, containerName, "echo", "network-ok")
	if err != nil {
		t.Errorf("exec failed: %v\n%s", err, out)
	}
}

func TestUpDown(t *testing.T) {
	requireContainerRuntime(t)
	fixture := "multi-service"

	// Up
	composeUp(t, fixture)

	// Verify running
	out, _ := compose(t, fixture, "ps")
	if !strings.Contains(out, "backend") {
		t.Fatal("backend not running after up")
	}

	// Down
	composeDown(t, fixture)

	// Verify stopped — ps should show "No containers running"
	out, _ = compose(t, fixture, "ps")
	if strings.Contains(out, "backend") && !strings.Contains(out, "No containers") {
		t.Error("services still listed after down")
	}
}

func TestWordPress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping WordPress test in short mode (heavy images)")
	}
	requireContainerRuntime(t)
	fixture := "wordpress"
	defer composeDown(t, fixture)

	composeUp(t, fixture)

	// Wait for MySQL to initialize
	t.Log("Waiting for MySQL to initialize...")
	var lastErr error
	for i := 0; i < 12; i++ {
		time.Sleep(5 * time.Second)

		resp, err := http.Get("http://localhost:8080")
		if err != nil {
			lastErr = err
			t.Logf("Attempt %d: %v", i+1, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusFound {
			t.Logf("WordPress responded with HTTP %d", resp.StatusCode)

			// Verify service discovery: WordPress can reach MySQL via 'db'
			out, err := containerExec(t, "wordpress-wordpress-1", "php", "-r",
				"echo gethostbyname('db');")
			if err != nil {
				t.Errorf("PHP DNS resolution failed: %v\n%s", err, out)
			} else {
				ip := strings.TrimSpace(out)
				if ip == "db" {
					t.Error("'db' hostname did not resolve (returned literal 'db')")
				} else {
					t.Logf("'db' resolves to %s", ip)
				}
			}
			return
		}

		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		t.Logf("Attempt %d: HTTP %d", i+1, resp.StatusCode)
	}

	t.Fatalf("WordPress did not become ready: %v", lastErr)
}

func TestLogs(t *testing.T) {
	requireContainerRuntime(t)
	fixture := "simple"
	defer composeDown(t, fixture)

	composeUp(t, fixture)

	out, err := compose(t, fixture, "logs", "hello")
	if err != nil {
		t.Logf("logs warning: %v", err)
	}
	// The simple container prints "hello from container-compose"
	t.Logf("Logs output:\n%s", out)
}
