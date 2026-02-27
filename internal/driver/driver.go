package driver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/apple/container-compose/internal/output"
)

const containerBinary = "container"

// Driver wraps the Apple `container` CLI.
type Driver struct {
	logger *output.Logger
}

// New creates a new Driver.
func New(logger *output.Logger) *Driver {
	return &Driver{logger: logger}
}

// ContainerInfo holds information about a running container.
type ContainerInfo struct {
	Name    string
	Service string
	Status  string
	Ports   string
	ID      string
}

// LogsOptions configures log streaming.
type LogsOptions struct {
	Follow bool
	Tail   string
}

// RunContainer executes `container run` with the given arguments.
func (d *Driver) RunContainer(ctx context.Context, args []string) error {
	d.logger.Debugf("Running: %s %s", containerBinary, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, containerBinary, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("container run failed: %w\n%s", err, stderr.String())
	}

	d.logger.Debugf("Container started: %s", strings.TrimSpace(string(out)))
	return nil
}

// StopContainer stops a running container.
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

// DeleteContainer removes a container.
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

// CreateNetwork creates a network.
func (d *Driver) CreateNetwork(ctx context.Context, name string) error {
	d.logger.Infof("Creating network %s", name)
	cmd := exec.CommandContext(ctx, containerBinary, "network", "create", name)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Ignore if already exists
		if strings.Contains(stderr.String(), "already exists") {
			d.logger.Debugf("Network %s already exists", name)
			return nil
		}
		return fmt.Errorf("network create %s failed: %w\n%s", name, err, stderr.String())
	}
	return nil
}

// DeleteNetwork removes a network.
func (d *Driver) DeleteNetwork(ctx context.Context, name string) error {
	d.logger.Infof("Removing network %s", name)
	cmd := exec.CommandContext(ctx, containerBinary, "network", "delete", name)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "not found") {
			return nil
		}
		return fmt.Errorf("network delete %s failed: %w\n%s", name, err, stderr.String())
	}
	return nil
}

// CreateVolume creates a named volume.
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

// DeleteVolume removes a named volume.
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

// ListContainers lists containers for a project by label.
func (d *Driver) ListContainers(ctx context.Context, projectName string) ([]ContainerInfo, error) {
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

	var rawContainers []map[string]interface{}
	if err := json.Unmarshal(out, &rawContainers); err != nil {
		// Try line-by-line JSON
		var containers []ContainerInfo
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			var raw map[string]interface{}
			if err := json.Unmarshal([]byte(line), &raw); err != nil {
				continue
			}
			name, _ := raw["name"].(string)
			if strings.HasPrefix(name, projectName+"-") {
				containers = append(containers, ContainerInfo{
					Name:    name,
					Service: extractServiceFromName(name, projectName),
					Status:  fmt.Sprintf("%v", raw["status"]),
				})
			}
		}
		return containers, nil
	}

	var containers []ContainerInfo
	for _, raw := range rawContainers {
		name, _ := raw["name"].(string)
		if strings.HasPrefix(name, projectName+"-") {
			containers = append(containers, ContainerInfo{
				Name:    name,
				Service: extractServiceFromName(name, projectName),
				Status:  fmt.Sprintf("%v", raw["status"]),
			})
		}
	}

	return containers, nil
}

// Logs streams logs from containers.
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

// BuildOptions configures image build operations.
type BuildOptions struct {
	Dockerfile string
	Args       map[string]*string
	Target     string
	CacheFrom  []string
	NoCache    bool
}

// BuildImage builds an image from a Dockerfile context.
func (d *Driver) BuildImage(ctx context.Context, contextPath string, dockerfile string, tag string) error {
	return d.BuildImageWithOptions(ctx, contextPath, tag, BuildOptions{Dockerfile: dockerfile})
}

// BuildImageWithOptions builds an image with extended options.
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

// ExecOptions configures exec operations.
type ExecOptions struct {
	Detach  bool
	User    string
	Workdir string
}

// ExecContainer executes a command in a running container.
func (d *Driver) ExecContainer(ctx context.Context, containerName string, command []string, opts ExecOptions) error {
	args := []string{"exec"}

	if opts.Detach {
		args = append(args, "-d")
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
	cmd := exec.CommandContext(ctx, containerBinary, args...)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()
	cmd.Stdin = nil // TODO: support interactive mode

	return cmd.Run()
}

func extractServiceFromName(containerName, projectName string) string {
	// Container name format: project-service-replica
	trimmed := strings.TrimPrefix(containerName, projectName+"-")
	parts := strings.Split(trimmed, "-")
	if len(parts) >= 2 {
		// Rejoin all parts except the last (replica number)
		return strings.Join(parts[:len(parts)-1], "-")
	}
	return trimmed
}

// KillContainer sends a signal to a container.
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

// PullImage pulls an image from a registry.
func (d *Driver) PullImage(ctx context.Context, image string, platform string) error {
	args := []string{"image", "pull"}
	if platform != "" {
		args = append(args, "--platform", platform)
	}
	args = append(args, image)

	d.logger.Infof("Pulling %s", image)
	cmd := exec.CommandContext(ctx, containerBinary, args...)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()

	return cmd.Run()
}

// PushImage pushes an image to a registry.
func (d *Driver) PushImage(ctx context.Context, image string) error {
	d.logger.Infof("Pushing %s", image)
	cmd := exec.CommandContext(ctx, containerBinary, "image", "push", image)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()

	return cmd.Run()
}

// InspectContainer returns raw JSON inspect output for a container.
func (d *Driver) InspectContainer(ctx context.Context, name string) (map[string]interface{}, error) {
	cmd := exec.CommandContext(ctx, containerBinary, "inspect", name)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("container inspect %s failed: %w\n%s", name, err, stderr.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parsing inspect output: %w", err)
	}
	return result, nil
}

// StatsContainer shows resource usage stats for a container.
func (d *Driver) StatsContainer(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, containerBinary, "stats", name)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()

	return cmd.Run()
}

// CopyToContainer copies files to a container.
func (d *Driver) CopyToContainer(ctx context.Context, containerName, srcPath, dstPath string) error {
	// Format: container cp SRC CONTAINER:DST
	target := fmt.Sprintf("%s:%s", containerName, dstPath)
	d.logger.Debugf("Copy: %s -> %s", srcPath, target)

	cmd := exec.CommandContext(ctx, containerBinary, "cp", srcPath, target)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()

	return cmd.Run()
}

// CopyFromContainer copies files from a container.
func (d *Driver) CopyFromContainer(ctx context.Context, containerName, srcPath, dstPath string) error {
	source := fmt.Sprintf("%s:%s", containerName, srcPath)
	d.logger.Debugf("Copy: %s -> %s", source, dstPath)

	cmd := exec.CommandContext(ctx, containerBinary, "cp", source, dstPath)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()

	return cmd.Run()
}

// TopContainer shows running processes in a container.
func (d *Driver) TopContainer(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, containerBinary, "exec", name, "ps", "aux")
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()

	return cmd.Run()
}

// ImageList lists images matching a filter.
func (d *Driver) ImageList(ctx context.Context) ([]map[string]interface{}, error) {
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

	var images []map[string]interface{}
	if err := json.Unmarshal(out, &images); err != nil {
		// Try line-by-line
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			var img map[string]interface{}
			if err := json.Unmarshal([]byte(line), &img); err != nil {
				continue
			}
			images = append(images, img)
		}
	}
	return images, nil
}

// StartContainer starts a stopped container.
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

// RunContainer executes `container run` with the given arguments and
// optionally connects stdin/stdout for interactive use.
func (d *Driver) RunContainerInteractive(ctx context.Context, args []string) error {
	d.logger.Debugf("Running (interactive): %s %s", containerBinary, strings.Join(args, " "))
	cmd := exec.CommandContext(ctx, containerBinary, args...)
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()
	cmd.Stdin = nil

	return cmd.Run()
}

// AttachContainer connects stdin/stdout/stderr to a running container.
func (d *Driver) AttachContainer(ctx context.Context, name string) error {
	d.logger.Debugf("Attaching to container: %s", name)
	cmd := exec.CommandContext(ctx, containerBinary, "exec", "-it", name, "/bin/sh")
	cmd.Stdout = d.logger.Stdout()
	cmd.Stderr = d.logger.Stderr()
	cmd.Stdin = os.Stdin

	return cmd.Run()
}
