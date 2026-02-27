package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/apple/container-compose/internal/converter"
	"github.com/apple/container-compose/internal/driver"
)

// RestartPolicy defines how a container should be restarted.
type RestartPolicy string

const (
	RestartNo            RestartPolicy = "no"
	RestartAlways        RestartPolicy = "always"
	RestartOnFailure     RestartPolicy = "on-failure"
	RestartUnlessStopped RestartPolicy = "unless-stopped"
)

// restartMonitor watches containers and restarts them according to their policy.
type restartMonitor struct {
	driver *driver.Driver
}

func newRestartMonitor(d *driver.Driver) *restartMonitor {
	return &restartMonitor{driver: d}
}

// monitorAndRestart watches a container and restarts it if it exits, according to the policy.
func (rm *restartMonitor) monitorAndRestart(ctx context.Context, projectName, serviceName string, policy RestartPolicy, runArgs []string) {
	if policy == RestartNo || policy == "" {
		return
	}

	containerName := converter.ContainerName(projectName, serviceName, 1)
	backoff := time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			containers, err := rm.driver.ListContainers(ctx, projectName)
			if err != nil {
				continue
			}

			running := false
			for _, c := range containers {
				if c.Name == containerName && c.Status == "running" {
					running = true
					break
				}
			}

			if running {
				backoff = 5 * time.Second
				continue
			}

			// Container is not running — decide whether to restart
			switch policy {
			case RestartAlways, RestartUnlessStopped:
				_ = rm.driver.DeleteContainer(ctx, containerName)
				if err := rm.driver.RunContainer(ctx, runArgs); err != nil {
					backoff = min(backoff*2, 30*time.Second)
					continue
				}
				backoff = 5 * time.Second

			case RestartOnFailure:
				// Only restart if it exited with non-zero
				// Since we can't easily get exit code from `container` CLI,
				// treat any non-running state as a failure for now
				_ = rm.driver.DeleteContainer(ctx, containerName)
				if err := rm.driver.RunContainer(ctx, runArgs); err != nil {
					backoff = min(backoff*2, 30*time.Second)
					continue
				}
				backoff = 5 * time.Second
			}
		}
	}
}

// parseRestartPolicy converts a compose restart string to our RestartPolicy.
func parseRestartPolicy(policy string) RestartPolicy {
	switch policy {
	case "always":
		return RestartAlways
	case "on-failure":
		return RestartOnFailure
	case "unless-stopped":
		return RestartUnlessStopped
	case "no", "":
		return RestartNo
	default:
		return RestartNo
	}
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// formatRestartInfo returns a description of restart policy for logging.
func formatRestartInfo(policy string) string {
	if policy == "" || policy == "no" {
		return ""
	}
	return fmt.Sprintf(" (restart: %s)", policy)
}
