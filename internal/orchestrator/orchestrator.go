package orchestrator

import (
	"context"
	"fmt"

	"github.com/apple/container-compose/internal/converter"
	"github.com/apple/container-compose/internal/driver"
	"github.com/apple/container-compose/internal/output"
	"github.com/compose-spec/compose-go/v2/types"
)

// Orchestrator manages the lifecycle of a compose project.
type Orchestrator struct {
	driver    *driver.Driver
	logger    *output.Logger
	cancelMon context.CancelFunc // cancels restart monitors
}

// UpOptions configures the up operation.
type UpOptions struct {
	Detach bool
	Build  bool
}

// DownOptions configures the down operation.
type DownOptions struct {
	RemoveVolumes bool
	RemoveOrphans bool
}

// New creates a new Orchestrator.
func New(d *driver.Driver, logger *output.Logger) *Orchestrator {
	return &Orchestrator{driver: d, logger: logger}
}

// Up creates and starts all services in dependency order.
func (o *Orchestrator) Up(ctx context.Context, project *types.Project, opts UpOptions) error {
	// 1. Create networks
	if err := o.createNetworks(ctx, project); err != nil {
		return fmt.Errorf("creating networks: %w", err)
	}

	// 2. Create volumes
	if err := o.createVolumes(ctx, project); err != nil {
		return fmt.Errorf("creating volumes: %w", err)
	}

	// 3. Build images if requested
	if opts.Build {
		if err := o.buildImages(ctx, project); err != nil {
			return fmt.Errorf("building images: %w", err)
		}
	}

	// 4. Start services in dependency order
	order, err := dependencyOrder(project)
	if err != nil {
		return fmt.Errorf("resolving dependencies: %w", err)
	}

	// Separate long-lived context for restart monitors (cancelled on Down)
	monCtx, monCancel := context.WithCancel(context.Background())
	o.cancelMon = monCancel
	rm := newRestartMonitor(o.driver)

	for _, serviceName := range order {
		service := project.Services[serviceName]

		// Wait for dependencies (health check aware)
		if err := o.waitForDependencies(ctx, project.Name, service, serviceName); err != nil {
			monCancel()
			return fmt.Errorf("waiting for dependencies of %s: %w", serviceName, err)
		}

		restartInfo := formatRestartInfo(service.Restart)
		o.logger.Infof("Starting service %s%s", serviceName, restartInfo)

		args := converter.ContainerRunArgs(project.Name, service, serviceName, 1)
		if err := o.driver.RunContainer(ctx, args); err != nil {
			monCancel()
			return fmt.Errorf("starting service %s: %w", serviceName, err)
		}

		// Start restart monitor in background if policy is set
		policy := parseRestartPolicy(service.Restart)
		if policy != RestartNo {
			go rm.monitorAndRestart(monCtx, project.Name, serviceName, policy, args)
		}

		o.logger.Successf("Service %s started", serviceName)
	}

	return nil
}

// Down stops and removes all services, networks, and optionally volumes.
func (o *Orchestrator) Down(ctx context.Context, project *types.Project, opts DownOptions) error {
	// Cancel any active restart monitors
	if o.cancelMon != nil {
		o.cancelMon()
	}

	// 1. Stop and remove containers in reverse dependency order
	order, err := dependencyOrder(project)
	if err != nil {
		return fmt.Errorf("resolving dependencies: %w", err)
	}

	// Reverse order for teardown
	for i := len(order) - 1; i >= 0; i-- {
		serviceName := order[i]
		containerName := converter.ContainerName(project.Name, serviceName, 1)

		o.logger.Infof("Stopping service %s", serviceName)

		if err := o.driver.StopContainer(ctx, containerName); err != nil {
			o.logger.Warnf("Failed to stop %s: %v", serviceName, err)
		}

		if err := o.driver.DeleteContainer(ctx, containerName); err != nil {
			o.logger.Warnf("Failed to remove %s: %v", serviceName, err)
		}
	}

	// 2. Remove networks (including default)
	defaultNet := converter.NetworkName(project.Name, "default")
	for name := range project.Networks {
		networkName := converter.NetworkName(project.Name, name)
		if err := o.driver.DeleteNetwork(ctx, networkName); err != nil {
			o.logger.Warnf("Failed to remove network %s: %v", networkName, err)
		}
	}
	// Always try to remove the default network
	if err := o.driver.DeleteNetwork(ctx, defaultNet); err != nil {
		o.logger.Warnf("Failed to remove network %s: %v", defaultNet, err)
	}

	// 3. Remove volumes if requested
	if opts.RemoveVolumes {
		for name := range project.Volumes {
			volumeName := converter.VolumeName(project.Name, name)
			if err := o.driver.DeleteVolume(ctx, volumeName); err != nil {
				o.logger.Warnf("Failed to remove volume %s: %v", volumeName, err)
			}
		}
	}

	o.logger.Successf("Project %s stopped", project.Name)
	return nil
}

func (o *Orchestrator) createNetworks(ctx context.Context, project *types.Project) error {
	defaultNet := converter.NetworkName(project.Name, "default")

	// Check if any service needs the default network (no explicit networks defined)
	needsDefault := false
	for _, service := range project.Services {
		if len(service.Networks) == 0 {
			needsDefault = true
			break
		}
	}

	if needsDefault {
		if err := o.driver.CreateNetwork(ctx, defaultNet); err != nil {
			return err
		}
	}

	for name := range project.Networks {
		networkName := converter.NetworkName(project.Name, name)
		if err := o.driver.CreateNetwork(ctx, networkName); err != nil {
			return err
		}
	}
	return nil
}

func (o *Orchestrator) createVolumes(ctx context.Context, project *types.Project) error {
	for name := range project.Volumes {
		volumeName := converter.VolumeName(project.Name, name)
		if err := o.driver.CreateVolume(ctx, volumeName); err != nil {
			return err
		}
	}
	return nil
}

func (o *Orchestrator) buildImages(ctx context.Context, project *types.Project) error {
	for name, service := range project.Services {
		if service.Build == nil {
			continue
		}

		tag := service.Image
		if tag == "" {
			tag = fmt.Sprintf("%s-%s", project.Name, name)
		}

		contextPath := service.Build.Context
		if contextPath == "" {
			contextPath = "."
		}

		dockerfile := service.Build.Dockerfile
		if err := o.driver.BuildImage(ctx, contextPath, dockerfile, tag); err != nil {
			return fmt.Errorf("building %s: %w", name, err)
		}
	}
	return nil
}
