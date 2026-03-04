//go:build integration

// Package integration provides end-to-end tests that require a running
// Apple Container runtime. Run with:
//
//	go test -tags integration -v -timeout 600s ./internal/integration/
package integration

import (
	"context"
	"encoding/json"
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

func TestHostname(t *testing.T) {
	requireContainerRuntime(t)
	fixture := "hostname"
	defer composeDown(t, fixture)

	composeUp(t, fixture)

	clientContainer := "hostname-client-1"

	// Verify the hostname alias is in /etc/hosts
	out, err := containerExec(t, clientContainer, "cat", "/etc/hosts")
	if err != nil {
		t.Fatalf("cat /etc/hosts failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "myserver") {
		t.Errorf("/etc/hosts should contain 'myserver' hostname alias, got:\n%s", out)
	}

	// Verify client can resolve the hostname alias
	out, err = containerExec(t, clientContainer, "getent", "hosts", "myserver")
	if err != nil {
		t.Fatalf("getent hosts myserver failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "myserver") {
		t.Errorf("getent should resolve 'myserver', got: %s", out)
	}

	// Verify the server's /etc/hostname is set
	serverContainer := "hostname-server-1"
	out, err = containerExec(t, serverContainer, "cat", "/etc/hostname")
	if err != nil {
		t.Logf("cat /etc/hostname warning (may not exist): %v", err)
	} else if !strings.Contains(out, "myserver") {
		t.Errorf("/etc/hostname should contain 'myserver', got: %s", out)
	}
}

func TestHostDockerInternal(t *testing.T) {
	requireContainerRuntime(t)
	fixture := "simple"
	defer composeDown(t, fixture)

	composeUp(t, fixture)

	containerName := "simple-hello-1"

	// Verify host.docker.internal is in /etc/hosts
	out, err := containerExec(t, containerName, "cat", "/etc/hosts")
	if err != nil {
		t.Fatalf("cat /etc/hosts failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "host.docker.internal") {
		t.Errorf("/etc/hosts should contain host.docker.internal, got:\n%s", out)
	}
	if !strings.Contains(out, "gateway.docker.internal") {
		t.Errorf("/etc/hosts should contain gateway.docker.internal, got:\n%s", out)
	}

	// Verify host.docker.internal resolves to a valid IP
	out, err = containerExec(t, containerName, "getent", "hosts", "host.docker.internal")
	if err != nil {
		t.Fatalf("getent hosts host.docker.internal failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "host.docker.internal") {
		t.Errorf("getent should resolve host.docker.internal, got: %s", out)
	}
}

func TestShmSize(t *testing.T) {
	requireContainerRuntime(t)
	fixture := "hostname"
	defer composeDown(t, fixture)

	composeUp(t, fixture)

	bigmemContainer := "hostname-bigmem-1"

	// Verify /dev/shm is remounted with the correct size (256MB)
	out, err := containerExec(t, bigmemContainer, "df", "-m", "/dev/shm")
	if err != nil {
		t.Fatalf("df /dev/shm failed: %v\n%s", err, out)
	}
	t.Logf("/dev/shm mount info:\n%s", out)

	// Should show 256MB, not the default 64MB
	if !strings.Contains(out, "256") {
		t.Errorf("expected /dev/shm to be 256MB, got:\n%s", out)
	}

	// Verify we can write to /dev/shm
	out, err = containerExec(t, bigmemContainer, "sh", "-c", "echo test > /dev/shm/testfile && cat /dev/shm/testfile")
	if err != nil {
		t.Errorf("failed to write to /dev/shm: %v", err)
	}
	if !strings.Contains(out, "test") {
		t.Errorf("expected 'test' from /dev/shm read, got: %s", out)
	}
}

func TestFullFeatures(t *testing.T) {
	requireContainerRuntime(t)
	fixture := "full-features"
	defer composeDown(t, fixture)

	composeUp(t, fixture)

	// 1. container_name: verify custom names are used
	out, err := containerExec(t, "test-db", "echo", "alive")
	if err != nil {
		t.Fatalf("container_name 'test-db' not reachable: %v\n%s", err, out)
	}

	out, err = containerExec(t, "test-app", "echo", "alive")
	if err != nil {
		t.Fatalf("container_name 'test-app' not reachable: %v\n%s", err, out)
	}

	// 2. depends_on service_healthy: app started only after db healthcheck passed
	//    If we got here, it means up succeeded and db was healthy before app started

	// 3. user: verify UID
	out, err = containerExec(t, "test-db", "id", "-u")
	if err != nil {
		t.Fatalf("id -u failed: %v\n%s", err, out)
	}
	if !strings.Contains(strings.TrimSpace(out), "1000") {
		t.Errorf("expected user 1000, got: %s", out)
	}

	// 4. read_only: app has read-only rootfs, writing should fail
	_, err = containerExec(t, "test-app", "sh", "-c", "touch /testfile 2>&1")
	if err == nil {
		t.Error("expected write to fail on read-only rootfs")
	}

	// 5. anonymous volumes: /var/run should be writable even with read_only
	_, err = containerExec(t, "test-app", "sh", "-c", "touch /var/run/testfile")
	if err != nil {
		t.Errorf("expected /var/run (tmpfs) to be writable on read_only container: %v", err)
	}

	// 6. named volume: db-data should be mounted
	_, err = containerExec(t, "test-db", "sh", "-c", "echo test > /data/voltest && cat /data/voltest")
	if err != nil {
		t.Errorf("named volume /data not writable: %v", err)
	}

	// 7. environment: verify env vars
	out, err = containerExec(t, "test-app", "printenv", "APP_MODE")
	if err != nil {
		t.Fatalf("printenv failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != "production" {
		t.Errorf("expected APP_MODE=production, got: %s", out)
	}

	// 8. host.docker.internal: GATEWAY env var should resolve
	out, err = containerExec(t, "test-app", "printenv", "GATEWAY")
	if err != nil {
		t.Fatalf("printenv GATEWAY failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != "host.docker.internal" {
		t.Errorf("expected GATEWAY=host.docker.internal, got: %s", out)
	}

	// Verify host.docker.internal resolves
	out, err = containerExec(t, "test-app", "getent", "hosts", "host.docker.internal")
	if err != nil {
		t.Errorf("host.docker.internal not resolvable: %v\n%s", err, out)
	}

	// 9. hostname: 'myapp' should be resolvable from other containers
	out, err = containerExec(t, "test-db", "getent", "hosts", "myapp")
	if err != nil {
		t.Errorf("hostname 'myapp' not resolvable from db: %v\n%s", err, out)
	}

	// 10. service discovery: containers resolve each other by service name
	out, err = containerExec(t, "test-app", "getent", "hosts", "db")
	if err != nil {
		t.Errorf("service name 'db' not resolvable from app: %v\n%s", err, out)
	}

	// Also by container_name
	out, err = containerExec(t, "test-app", "getent", "hosts", "test-db")
	if err != nil {
		t.Errorf("container_name 'test-db' not resolvable from app: %v\n%s", err, out)
	}

	// 11. shm_size: verify /dev/shm is remounted to 128MB
	out, err = containerExec(t, "test-db", "df", "-m", "/dev/shm")
	if err != nil {
		t.Fatalf("df /dev/shm failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "128") {
		t.Errorf("expected /dev/shm at 128MB, got:\n%s", out)
	}

	// 12. depends_on service_started: worker should be running
	workerContainer := "full-features-worker-1"
	out, err = containerExec(t, workerContainer, "echo", "alive")
	if err != nil {
		t.Errorf("worker not running (depends_on service_started): %v\n%s", err, out)
	}

	// 13. env_file: worker should have env vars from worker.env
	out, err = containerExec(t, workerContainer, "printenv", "WORKER_MODE")
	if err != nil {
		t.Errorf("printenv WORKER_MODE failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != "background" {
		t.Errorf("expected WORKER_MODE=background from env_file, got: %s", out)
	}

	// 14. command: db's command should have created /tmp/health
	out, err = containerExec(t, "test-db", "cat", "/tmp/health")
	if err != nil {
		t.Errorf("command did not run (expected /tmp/health): %v", err)
	}
	if !strings.Contains(out, "ready") {
		t.Errorf("expected 'ready' in /tmp/health, got: %s", out)
	}
}

