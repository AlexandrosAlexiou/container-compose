// Package orchestrator manages the lifecycle of a compose project, including dependency ordering,
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/apple/container-compose/internal/converter"
	"github.com/apple/container-compose/internal/driver"
	"github.com/compose-spec/compose-go/v2/types"
)

func waitForHealthy(ctx context.Context, d *driver.Driver, containerName string, healthcheck *types.HealthCheckConfig, timeout time.Duration) error {
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	interval := 2 * time.Second
	if healthcheck != nil && healthcheck.Interval != nil {
		interval = time.Duration(*healthcheck.Interval)
		if interval < time.Second {
			interval = 2 * time.Second
		}
	}
	if healthcheck != nil && healthcheck.StartPeriod != nil {
		startPeriod := time.Duration(*healthcheck.StartPeriod)
		if startPeriod > 0 {
			time.Sleep(startPeriod)
		}
	}

	retries := 30
	if healthcheck != nil && healthcheck.Retries != nil {
		retries = int(*healthcheck.Retries)
	}

	deadline := time.After(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timed out waiting for %s to become healthy", containerName)
		case <-ticker.C:
			attempt++
			if attempt > retries {
				return fmt.Errorf("%s failed health check after %d retries", containerName, retries)
			}

			healthy, err := checkHealthExec(ctx, d, containerName, healthcheck)
			if err != nil {
				continue
			}
			if healthy {
				return nil
			}
		}
	}
}

func checkHealthExec(ctx context.Context, d *driver.Driver, containerName string, healthcheck *types.HealthCheckConfig) (bool, error) {
	if healthcheck != nil && len(healthcheck.Test) > 0 {
		var cmd []string
		switch healthcheck.Test[0] {
		case "CMD":
			cmd = healthcheck.Test[1:]
		case "CMD-SHELL":
			cmd = []string{"sh", "-c", strings.Join(healthcheck.Test[1:], " ")}
		default:
			cmd = healthcheck.Test
		}

		if len(cmd) > 0 {
			_, err := d.ExecSimple(ctx, containerName, cmd)
			return err == nil, nil
		}
	}

	// Fallback: check container status is "running"
	return checkHealth(ctx, containerName)
}

func checkHealth(ctx context.Context, containerName string) (bool, error) {
	cmd := exec.CommandContext(ctx, "container", "inspect", containerName)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}

	var status string
	var arr []map[string]any
	if err := json.Unmarshal(out, &arr); err == nil && len(arr) > 0 {
		status, _ = arr[0]["status"].(string)
	} else {
		var result map[string]any
		if err := json.Unmarshal(out, &result); err != nil {
			return false, err
		}
		status, _ = result["status"].(string)
	}

	return strings.EqualFold(status, "running"), nil
}

func shouldWaitForHealthy(dep types.ServiceDependency) bool {
	return dep.Condition == "service_healthy"
}

func (o *Orchestrator) waitForDependencies(ctx context.Context, projectName string, service types.ServiceConfig) error {
	for depName, dep := range service.DependsOn {
		depService := o.findService(depName)
		containerName := converter.ContainerName(projectName, depName, 1)
		if depService != nil && depService.ContainerName != "" {
			containerName = depService.ContainerName
		}

		if shouldWaitForHealthy(dep) {
			o.logger.Infof("Waiting for %s to be healthy...", depName)
			timeout := 120 * time.Second

			var healthcheck *types.HealthCheckConfig
			if depService != nil {
				healthcheck = depService.HealthCheck
			}

			if err := waitForHealthy(ctx, o.driver, containerName, healthcheck, timeout); err != nil {
				return fmt.Errorf("dependency %s: %w", depName, err)
			}
			o.logger.Successf("Dependency %s is healthy", depName)
		} else {
			o.logger.Debugf("Dependency %s started (no health wait)", depName)
		}
	}
	return nil
}
