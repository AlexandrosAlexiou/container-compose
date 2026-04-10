// Package driver wraps the Apple Container CLI for container, network, volume, and registry operations.
package driver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/apple/container-compose/internal/output"
)

const containerBinary = "container"

type Driver struct {
	logger *output.Logger
}

func New(logger *output.Logger) *Driver {
	return &Driver{logger: logger}
}

type ContainerInfo struct {
	Name    string
	Service string
	Status  string
	Ports   string
	ID      string
}

type LogsOptions struct {
	Follow bool
	Tail   string
}

func (d *Driver) RunContainer(ctx context.Context, args []string) error {
	return d.RunContainerWithWriter(ctx, args, nil)
}

// RunContainerWithWriter runs a container, directing subprocess output to the
// given writer. If w is nil, stderr goes to the logger's stderr (raw).
// When w is provided, both stdout and stderr are sent through it so that
// image-pull progress is captured regardless of which stream the container
// CLI uses.
func (d *Driver) RunContainerWithWriter(ctx context.Context, args []string, w io.Writer) error {
	d.logger.Debugf("Running: %s %s", containerBinary, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, containerBinary, args...)

	if w != nil {
		cmd.Stdout = w
		cmd.Stderr = w
	} else {
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = d.logger.Stderr()

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("container run failed: %w", err)
		}
		d.logger.Debugf("Container started: %s", strings.TrimSpace(stdout.String()))
		return nil
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("container run failed: %w", err)
	}
	return nil
}

func (d *Driver) StopContainer(ctx context.Context, name string) error {
	d.logger.Debugf("Stopping container: %s", name)
	cmd := exec.CommandContext(ctx, containerBinary, "stop", name)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("container stop %s failed: %w\n%s", name, err, stderr.String())
	}
	return nil
}

func (d *Driver) DeleteContainer(ctx context.Context, name string) error {
	d.logger.Debugf("Deleting container: %s", name)
	cmd := exec.CommandContext(ctx, containerBinary, "delete", name)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("container delete %s failed: %w\n%s", name, err, stderr.String())
	}
	return nil
}

func (d *Driver) ForceDeleteContainer(ctx context.Context, name string) error {
	d.logger.Debugf("Force deleting container: %s", name)
	cmd := exec.CommandContext(ctx, containerBinary, "delete", "--force", name)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("container delete --force %s failed: %w\n%s", name, err, stderr.String())
	}
	return nil
}

func (d *Driver) CreateNetwork(ctx context.Context, name string) error {
	d.logger.Infof("Creating network %s", name)

	for attempt := 0; attempt < 5; attempt++ {
		cmd := exec.CommandContext(ctx, containerBinary, "network", "create", name)

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			errMsg := stderr.String()
			if strings.Contains(errMsg, "already exists") {
				d.logger.Debugf("Network %s already exists", name)
				return nil
			}
			if strings.Contains(errMsg, "pending operation") {
				d.logger.Debugf("Network %s has pending operation, waiting...", name)
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			return fmt.Errorf("network create %s failed: %w\n%s", name, err, errMsg)
		}
		return nil
	}
	return fmt.Errorf("network create %s failed: pending operation did not resolve after retries", name)
}

func (d *Driver) DeleteNetwork(ctx context.Context, name string) error {
	d.logger.Infof("Removing network %s", name)
	cmd := exec.CommandContext(ctx, containerBinary, "network", "delete", name)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "not found") {
			return nil
		}
		return fmt.Errorf("network delete %s failed: %w\n%s", name, err, errMsg)
	}
	return nil
}

func (d *Driver) ListNetworks(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, containerBinary, "network", "list")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("network list failed: %w\n%s", err, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var networks []string
	for i, line := range lines {
		// Skip the header row
		if i == 0 && strings.Contains(line, "NETWORK") {
			continue
		}
		// Extract first column (network name)
		fields := strings.Fields(line)
		if len(fields) > 0 {
			networks = append(networks, fields[0])
		}
	}
	return networks, nil
}

func (d *Driver) ListNetworksRaw(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, containerBinary, "network", "list")
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()
	return cmd.Run()
}

func (d *Driver) CreateVolume(ctx context.Context, name string) error {
	d.logger.Infof("Creating volume %s", name)
	cmd := exec.CommandContext(ctx, containerBinary, "volume", "create", name)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "already exists") {
			d.logger.Debugf("Volume %s already exists", name)
			return nil
		}
		return fmt.Errorf("volume create %s failed: %w\n%s", name, err, stderr.String())
	}
	return nil
}

func (d *Driver) DeleteVolume(ctx context.Context, name string) error {
	d.logger.Infof("Removing volume %s", name)
	cmd := exec.CommandContext(ctx, containerBinary, "volume", "delete", name)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "not found") {
			return nil
		}
		return fmt.Errorf("volume delete %s failed: %w\n%s", name, err, stderr.String())
	}
	return nil
}

func (d *Driver) ListContainers(ctx context.Context, projectName string, customNames ...map[string]string) ([]ContainerInfo, error) {
	cmd := exec.CommandContext(ctx, containerBinary, "list", "--format", "json")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("container list failed: %w\n%s", err, stderr.String())
	}

	if len(strings.TrimSpace(string(out))) == 0 {
		return nil, nil
	}

	var rawContainers []map[string]any
	if err := json.Unmarshal(out, &rawContainers); err != nil {
		return nil, fmt.Errorf("parsing container list: %w", err)
	}

	// Build reverse lookup: custom container name -> service name
	nameToService := make(map[string]string)
	if len(customNames) > 0 {
		for svc, cn := range customNames[0] {
			nameToService[cn] = svc
		}
	}

	var containers []ContainerInfo
	for _, raw := range rawContainers {
		// Apple Container JSON: name is at configuration.id, status at top level
		name := ""
		status := ""
		ports := ""

		if config, ok := raw["configuration"].(map[string]any); ok {
			name, _ = config["id"].(string)
			ports = formatPublishedPorts(config)

			// Append exposed-only ports from label (Docker Compose compat)
			if exposed := getLabel(config, "com.docker.compose.expose"); exposed != "" {
				exposedPorts := formatExposedPorts(exposed, ports)
				if exposedPorts != "" {
					if ports != "" {
						ports += ", "
					}
					ports += exposedPorts
				}
			}
		}
		// Fallback: try "name" at top level
		if name == "" {
			name, _ = raw["name"].(string)
		}
		status, _ = raw["status"].(string)

		if name == "" {
			continue
		}

		// Match by project prefix (generated names like "local-postgres-1")
		if strings.HasPrefix(name, projectName+"-") {
			containers = append(containers, ContainerInfo{
				Name:    name,
				Service: extractServiceFromName(name, projectName),
				Status:  status,
				Ports:   ports,
			})
			continue
		}

		// Match by explicit container_name from compose file
		if svc, ok := nameToService[name]; ok {
			containers = append(containers, ContainerInfo{
				Name:    name,
				Service: svc,
				Status:  status,
				Ports:   ports,
			})
		}
	}

	return containers, nil
}

func (d *Driver) Logs(ctx context.Context, projectName string, services []string, opts LogsOptions) error {
	for _, service := range services {
		containerName := fmt.Sprintf("%s-%s-1", projectName, service)

		args := []string{"logs"}
		if opts.Follow {
			args = append(args, "-f")
		}
		args = append(args, containerName)

		d.logger.Infof("Logs for %s:", service)
		cmd := exec.CommandContext(ctx, containerBinary, args...)
		cmd.Stdout = d.logger.Stdout()
		cmd.Stderr = d.logger.Stderr()

		if err := cmd.Run(); err != nil {
			d.logger.Warnf("Failed to get logs for %s: %v", service, err)
		}
	}
	return nil
}

// ANSI color codes for service log prefixes.
var serviceColors = []string{
	"\033[32m", // green
	"\033[33m", // yellow
	"\033[34m", // blue
	"\033[35m", // magenta
	"\033[36m", // cyan
	"\033[91m", // bright red
	"\033[92m", // bright green
	"\033[93m", // bright yellow
	"\033[94m", // bright blue
	"\033[95m", // bright magenta
}

const colorReset = "\033[0m"

// FollowLogs streams logs from multiple containers concurrently, prefixing each
// line with a color-coded service name, similar to `docker compose up`.
// serviceContainers maps service display names to actual container names.
// It blocks until ctx is cancelled.
func (d *Driver) FollowLogs(ctx context.Context, serviceContainers map[string]string, w io.Writer) {
	// Compute max service name length for aligned output
	maxLen := 0
	for s := range serviceContainers {
		if len(s) > maxLen {
			maxLen = len(s)
		}
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	i := 0
	for service, containerName := range serviceContainers {
		color := serviceColors[i%len(serviceColors)]
		prefix := fmt.Sprintf("%s%-*s |%s ", color, maxLen, service, colorReset)
		i++

		wg.Add(1)
		go func(container, pfx string) {
			defer wg.Done()
			d.streamLogs(ctx, container, pfx, w, &mu)
		}(containerName, prefix)
	}

	wg.Wait()
}

func (d *Driver) streamLogs(ctx context.Context, containerName, prefix string, w io.Writer, mu *sync.Mutex) {
	for {
		if ctx.Err() != nil {
			return
		}

		cmd := exec.CommandContext(ctx, containerBinary, "logs", "-f", containerName)

		pr, pw := io.Pipe()
		cmd.Stdout = pw
		// Capture CLI errors separately so they don't appear as container output
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			if !d.IsContainerRunning(ctx, containerName) {
				return
			}
			d.logger.Debugf("Retrying log follow for %s: %v", containerName, err)
			time.Sleep(2 * time.Second)
			continue
		}

		go func() {
			_ = cmd.Wait()
			pw.Close()
		}()

		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			mu.Lock()
			fmt.Fprintf(w, "%s%s\n", prefix, scanner.Text())
			mu.Unlock()
		}

		// Log stream ended — only retry if the container is still alive
		if ctx.Err() != nil || !d.IsContainerRunning(ctx, containerName) {
			return
		}
		if errMsg := stderr.String(); errMsg != "" {
			d.logger.Debugf("Log follow for %s failed: %s", containerName, strings.TrimSpace(errMsg))
		}
		stderr.Reset()
		time.Sleep(2 * time.Second)
	}
}

func (d *Driver) IsContainerRunning(ctx context.Context, name string) bool {
	info, err := d.InspectContainer(ctx, name)
	if err != nil {
		return false
	}
	status, _ := info["status"].(string)
	return strings.EqualFold(status, "running")
}

type BuildOptions struct {
	Dockerfile string
	Args       map[string]*string
	Target     string
	CacheFrom  []string
	NoCache    bool
}

func (d *Driver) BuildImage(ctx context.Context, contextPath string, dockerfile string, tag string) error {
	return d.BuildImageWithOptions(ctx, contextPath, tag, BuildOptions{Dockerfile: dockerfile})
}

func (d *Driver) BuildImageWithOptions(ctx context.Context, contextPath string, tag string, opts BuildOptions) error {
	args := []string{"image", "build", "-t", tag}
	if opts.Dockerfile != "" {
		args = append(args, "-f", opts.Dockerfile)
	}
	if opts.Target != "" {
		args = append(args, "--target", opts.Target)
	}
	if opts.NoCache {
		args = append(args, "--no-cache")
	}
	for _, cache := range opts.CacheFrom {
		args = append(args, "--cache-from", cache)
	}
	for k, v := range opts.Args {
		if v != nil {
			args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, *v))
		} else {
			args = append(args, "--build-arg", k)
		}
	}
	args = append(args, contextPath)

	d.logger.Infof("Building image %s", tag)
	cmd := exec.CommandContext(ctx, containerBinary, args...)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()

	return cmd.Run()
}

type ExecOptions struct {
	Detach      bool
	User        string
	Workdir     string
	Interactive bool
	TTY         bool
}

func (d *Driver) ExecContainer(ctx context.Context, containerName string, command []string, opts ExecOptions) error {
	args := []string{"exec"}

	if opts.Detach {
		args = append(args, "-d")
	}
	if opts.Interactive {
		args = append(args, "-i")
	}
	if opts.TTY {
		args = append(args, "-t")
	}
	if opts.User != "" {
		args = append(args, "-u", opts.User)
	}
	if opts.Workdir != "" {
		args = append(args, "-w", opts.Workdir)
	}

	args = append(args, containerName)
	args = append(args, command...)

	d.logger.Debugf("Exec: %s %s", containerBinary, strings.Join(args, " "))

	// For interactive sessions, replace the current process with the container
	// binary so it gets direct access to the terminal (PTY, paste buffer, etc.).
	if opts.Interactive && !opts.Detach {
		binary, err := exec.LookPath(containerBinary)
		if err != nil {
			return fmt.Errorf("finding %s binary: %w", containerBinary, err)
		}
		return syscall.Exec(binary, append([]string{containerBinary}, args...), os.Environ())
	}

	cmd := exec.CommandContext(ctx, containerBinary, args...)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()

	return cmd.Run()
}

func extractServiceFromName(containerName, projectName string) string {
	trimmed := strings.TrimPrefix(containerName, projectName+"-")
	parts := strings.Split(trimmed, "-")
	if len(parts) >= 2 {
		return strings.Join(parts[:len(parts)-1], "-")
	}
	return trimmed
}

// formatPublishedPorts extracts port mappings from the configuration JSON.
// Format: "0.0.0.0:8080->80/tcp"
func formatPublishedPorts(config map[string]any) string {
	portsRaw, ok := config["publishedPorts"].([]any)
	if !ok || len(portsRaw) == 0 {
		return ""
	}

	var parts []string
	for _, p := range portsRaw {
		port, ok := p.(map[string]any)
		if !ok {
			continue
		}
		hostAddr, _ := port["hostAddress"].(string)
		if hostAddr == "" {
			hostAddr = "0.0.0.0"
		}
		hostPort := intFromJSON(port["hostPort"])
		containerPort := intFromJSON(port["containerPort"])
		proto, _ := port["proto"].(string)
		if proto == "" {
			proto = "tcp"
		}

		if hostPort > 0 && containerPort > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d->%d/%s", hostAddr, hostPort, containerPort, proto))
		}
	}
	return strings.Join(parts, ", ")
}

func intFromJSON(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// getLabel extracts a label value from the Apple Container configuration JSON.
func getLabel(config map[string]any, key string) string {
	labels, ok := config["labels"].(map[string]any)
	if !ok {
		return ""
	}
	val, _ := labels[key].(string)
	return val
}

// formatExposedPorts returns exposed-only ports (not already published) as "port/proto".
func formatExposedPorts(exposed, publishedPorts string) string {
	var parts []string
	for _, e := range strings.Split(exposed, ",") {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		// Parse port/proto; default to tcp
		port := e
		proto := "tcp"
		if i := strings.Index(e, "/"); i >= 0 {
			port = e[:i]
			proto = e[i+1:]
		}
		// Skip if this port is already shown as a published port
		if strings.Contains(publishedPorts, "->"+port+"/"+proto) {
			continue
		}
		parts = append(parts, port+"/"+proto)
	}
	return strings.Join(parts, ", ")
}

func (d *Driver) KillContainer(ctx context.Context, name string, signal string) error {
	args := []string{"kill"}
	if signal != "" {
		args = append(args, "-s", signal)
	}
	args = append(args, name)

	d.logger.Debugf("Kill: %s %s", containerBinary, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, containerBinary, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("container kill %s failed: %w\n%s", name, err, stderr.String())
	}
	return nil
}

func (d *Driver) RegistryLogin(ctx context.Context, server, username, secret string) error {
	args := []string{"registry", "login"}
	if username != "" {
		args = append(args, "--username", username)
	}
	args = append(args, "--password-stdin", server)

	d.logger.Debugf("Registry login: %s (user=%s)", server, username)
	cmd := exec.CommandContext(ctx, containerBinary, args...)
	cmd.Stdin = strings.NewReader(secret)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("registry login to %s failed: %w\n%s", server, err, stderr.String())
	}
	return nil
}

func (d *Driver) IsRegistryLoggedIn(ctx context.Context, server string) bool {
	cmd := exec.CommandContext(ctx, containerBinary, "registry", "list")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), server)
}

func (d *Driver) PullImage(ctx context.Context, image string, platform string) error {
	return d.PullImageWithWriter(ctx, image, platform, nil)
}

// PullImageWithWriter pulls an image, directing subprocess output to the
// given writer. If w is nil, a default single-line ProgressWriter is used.
// When w is provided, the command is wrapped in a PTY (via script(1)) so
// that the container CLI emits its rich ANSI progress output even though
// stdout/stderr are pipes.
func (d *Driver) PullImageWithWriter(ctx context.Context, image string, platform string, w io.Writer) error {
	args := []string{"image", "pull"}
	if platform != "" {
		args = append(args, "--platform", platform)
	}
	args = append(args, image)

	if w == nil {
		d.logger.Infof("Pulling %s", image)
		pw := output.NewProgressWriter(d.logger.Stderr())
		cmd := exec.CommandContext(ctx, containerBinary, args...)
		cmd.Stdout = pw
		cmd.Stderr = pw
		err := cmd.Run()
		pw.Finish()
		return err
	}

	// Wrap in script(1) to allocate a PTY. The container CLI detects
	// non-TTY pipes and suppresses progress output; script forces TTY.
	scriptArgs := []string{"-q", "/dev/null"}
	scriptArgs = append(scriptArgs, containerBinary)
	scriptArgs = append(scriptArgs, args...)
	cmd := exec.CommandContext(ctx, "script", scriptArgs...)
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

func (d *Driver) PushImage(ctx context.Context, image string) error {
	d.logger.Infof("Pushing %s", image)

	pw := output.NewProgressWriter(d.logger.Stderr())
	cmd := exec.CommandContext(ctx, containerBinary, "image", "push", image)
	cmd.Stdout = pw
	cmd.Stderr = pw

	err := cmd.Run()
	pw.Finish()
	return err
}

func (d *Driver) InspectContainer(ctx context.Context, name string) (map[string]any, error) {
	cmd := exec.CommandContext(ctx, containerBinary, "inspect", name)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("container inspect %s failed: %w\n%s", name, err, stderr.String())
	}

	// Apple Container inspect returns a JSON array with one element
	var arr []map[string]any
	if err := json.Unmarshal(out, &arr); err != nil {
		// Fallback: try parsing as single object
		var result map[string]any
		if err2 := json.Unmarshal(out, &result); err2 != nil {
			return nil, fmt.Errorf("parsing inspect output: %w", err)
		}
		return result, nil
	}
	if len(arr) == 0 {
		return nil, fmt.Errorf("inspect returned empty result for %s", name)
	}
	return arr[0], nil
}

func (d *Driver) GetContainerIP(ctx context.Context, name string) (string, error) {
	info, err := d.InspectContainer(ctx, name)
	if err != nil {
		return "", err
	}

	networks, ok := info["networks"].([]any)
	if !ok || len(networks) == 0 {
		return "", fmt.Errorf("no networks found for container %s", name)
	}

	net, ok := networks[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid network info for container %s", name)
	}

	ipCIDR, _ := net["ipv4Address"].(string)
	if ipCIDR == "" {
		return "", fmt.Errorf("no IPv4 address for container %s", name)
	}

	// Strip CIDR notation (e.g., "192.168.64.2/24" -> "192.168.64.2")
	ip := strings.Split(ipCIDR, "/")[0]
	return ip, nil
}

// GetContainerGateway returns the IPv4 gateway of a container's network.
// This is the host IP from the container's perspective.
func (d *Driver) GetContainerGateway(ctx context.Context, name string) (string, error) {
	info, err := d.InspectContainer(ctx, name)
	if err != nil {
		return "", err
	}

	networks, ok := info["networks"].([]any)
	if !ok || len(networks) == 0 {
		return "", fmt.Errorf("no networks found for container %s", name)
	}

	net, ok := networks[0].(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid network info for container %s", name)
	}

	gw, _ := net["ipv4Gateway"].(string)
	if gw == "" {
		return "", fmt.Errorf("no gateway for container %s", name)
	}
	return gw, nil
}

func (d *Driver) ExecSimple(ctx context.Context, containerName string, command []string) (string, error) {
	args := []string{"exec", containerName}
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, containerBinary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("exec in %s failed: %w\n%s", containerName, err, stderr.String())
	}
	return stdout.String(), nil
}

func (d *Driver) StatsContainer(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, containerBinary, "stats", name)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()

	return cmd.Run()
}

func (d *Driver) CopyToContainer(ctx context.Context, containerName, srcPath, dstPath string) error {
	// Format: container cp SRC CONTAINER:DST
	target := fmt.Sprintf("%s:%s", containerName, dstPath)
	d.logger.Debugf("Copy: %s -> %s", srcPath, target)

	cmd := exec.CommandContext(ctx, containerBinary, "cp", srcPath, target)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()

	return cmd.Run()
}

func (d *Driver) CopyFromContainer(ctx context.Context, containerName, srcPath, dstPath string) error {
	source := fmt.Sprintf("%s:%s", containerName, srcPath)
	d.logger.Debugf("Copy: %s -> %s", source, dstPath)

	cmd := exec.CommandContext(ctx, containerBinary, "cp", source, dstPath)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()

	return cmd.Run()
}

func (d *Driver) TopContainer(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, containerBinary, "exec", name, "ps", "aux")
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()

	return cmd.Run()
}

func (d *Driver) DeleteImage(ctx context.Context, image string) error {
	d.logger.Debugf("Deleting image: %s", image)
	cmd := exec.CommandContext(ctx, containerBinary, "image", "delete", image)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("image delete %s failed: %w\n%s", image, err, stderr.String())
	}
	return nil
}

func (d *Driver) ImageList(ctx context.Context) ([]map[string]any, error) {
	cmd := exec.CommandContext(ctx, containerBinary, "image", "list", "--format", "json")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("image list failed: %w\n%s", err, stderr.String())
	}

	if len(strings.TrimSpace(string(out))) == 0 {
		return nil, nil
	}

	var images []map[string]any
	if err := json.Unmarshal(out, &images); err != nil {
		// Try line-by-line
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			var img map[string]any
			if err := json.Unmarshal([]byte(line), &img); err != nil {
				continue
			}
			images = append(images, img)
		}
	}
	return images, nil
}

func (d *Driver) StartContainer(ctx context.Context, name string) error {
	d.logger.Debugf("Starting container: %s", name)
	cmd := exec.CommandContext(ctx, containerBinary, "start", name)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("container start %s failed: %w\n%s", name, err, stderr.String())
	}
	return nil
}

func (d *Driver) RunContainerInteractive(ctx context.Context, args []string) error {
	d.logger.Debugf("Running (interactive): %s %s", containerBinary, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, containerBinary, args...)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()
	cmd.Stdin = nil

	return cmd.Run()
}

func (d *Driver) AttachContainer(ctx context.Context, name string) error {
	d.logger.Debugf("Attaching to container: %s", name)
	args := []string{"exec", "-it", name, "/bin/sh"}
	binary, err := exec.LookPath(containerBinary)
	if err != nil {
		return fmt.Errorf("finding %s binary: %w", containerBinary, err)
	}
	return syscall.Exec(binary, append([]string{containerBinary}, args...), os.Environ())
}
