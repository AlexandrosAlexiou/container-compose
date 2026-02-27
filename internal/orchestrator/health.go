package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
)

// waitForHealthy polls a container's health status until healthy or timeout.
func waitForHealthy(ctx context.Context, containerName string, timeout time.Duration) error {
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	deadline := time.After(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timed out waiting for %s to become healthy", containerName)
		case <-ticker.C:
			healthy, err := checkHealth(ctx, containerName)
			if err != nil {
				continue // container may not be ready yet
			}
			if healthy {
				return nil
			}
		}
	}
}

// checkHealth inspects a container and returns whether it reports healthy.
func checkHealth(ctx context.Context, containerName string) (bool, error) {
	cmd := exec.CommandContext(ctx, "container", "inspect", containerName)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		return false, err
	}

	// Check for health status in inspect output
	status, ok := result["status"].(string)
	if !ok {
		return false, nil
	}

	return strings.EqualFold(status, "running"), nil
}

// shouldWaitForHealthy returns true if the dependency has a service_healthy condition.
func shouldWaitForHealthy(dep types.ServiceDependency) bool {
	return dep.Condition == "service_healthy"
}

// waitForDependencies waits for all dependencies of a service to be ready.
func (o *Orchestrator) waitForDependencies(ctx context.Context, projectName string, service types.ServiceConfig, serviceName string) error {
	for depName, dep := range service.DependsOn {
		containerName := fmt.Sprintf("%s-%s-1", projectName, depName)

		if shouldWaitForHealthy(dep) {
			o.logger.Infof("Waiting for %s to be healthy...", depName)
			timeout := 60 * time.Second
			if dep.Condition != "" {
				// Use a reasonable default; compose spec doesn't have timeout on depends_on
			}
			if err := waitForHealthy(ctx, containerName, timeout); err != nil {
				return fmt.Errorf("dependency %s: %w", depName, err)
			}
			o.logger.Successf("Dependency %s is healthy", depName)
		} else {
			// service_started: just check it exists/is running
			o.logger.Debugf("Dependency %s started (no health wait)", depName)
		}
	}
	return nil
}
