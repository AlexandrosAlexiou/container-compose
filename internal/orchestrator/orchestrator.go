// Package orchestrator manages the lifecycle of a compose project, including dependency ordering,
package orchestrator

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/apple/container-compose/internal/converter"
	"github.com/apple/container-compose/internal/driver"
	"github.com/apple/container-compose/internal/output"
	"github.com/compose-spec/compose-go/v2/types"
)

type Orchestrator struct {
	driver    *driver.Driver
	logger    *output.Logger
	cancelMon context.CancelFunc // cancels restart monitors
	project   *types.Project     // current project (set during Up)
}

type UpOptions struct {
	Detach bool
	Build  bool
	Scale  map[string]int // service name -> replica count
}

type DownOptions struct {
	RemoveVolumes bool
	RemoveOrphans bool
}

func New(d *driver.Driver, logger *output.Logger) *Orchestrator {
	return &Orchestrator{driver: d, logger: logger}
}

func (o *Orchestrator) Up(ctx context.Context, project *types.Project, opts UpOptions) error {
	o.project = project

	if err := checkPortConflicts(project, opts.Scale); err != nil {
		return err
	}

	if err := o.createNetworks(ctx, project); err != nil {
		return fmt.Errorf("creating networks: %w", err)
	}

	if err := o.createVolumes(ctx, project); err != nil {
		return fmt.Errorf("creating volumes: %w", err)
	}

	if opts.Build {
		if err := o.buildImages(ctx, project); err != nil {
			return fmt.Errorf("building images: %w", err)
		}
	}

	levels, err := dependencyLevels(project)
	if err != nil {
		return fmt.Errorf("resolving dependencies: %w", err)
	}

	monCtx, monCancel := context.WithCancel(context.Background())
	o.cancelMon = monCancel
	rm := newRestartMonitor(o.driver)

	for _, level := range levels {
		var wg sync.WaitGroup
		errCh := make(chan error, len(level))

		for _, serviceName := range level {
			service := project.Services[serviceName]

			wg.Add(1)
			go func(svcName string, svc types.ServiceConfig) {
				defer wg.Done()

				if err := o.waitForDependencies(ctx, project.Name, svc); err != nil {
					errCh <- fmt.Errorf("waiting for dependencies of %s: %w", svcName, err)
					return
				}

				replicas := replicaCount(svcName, svc, opts.Scale)
				restartInfo := formatRestartInfo(svc.Restart)

				if len(svc.Networks) > 1 {
					o.logger.Warnf("Service %s has %d networks; only the first will be attached (Apple Container does not support post-create network connect)", svcName, len(svc.Networks))
				}

				for i := 1; i <= replicas; i++ {
					suffix := ""
					if replicas > 1 {
						suffix = fmt.Sprintf(" (%d/%d)", i, replicas)
					}
					o.logger.Infof("Starting service %s%s%s", svcName, suffix, restartInfo)

					containerName := converter.ContainerName(project.Name, svcName, i)
					if svc.ContainerName != "" {
						containerName = svc.ContainerName
					}

					_ = o.driver.StopContainer(ctx, containerName)
					_ = o.driver.ForceDeleteContainer(ctx, containerName)

					args := converter.ContainerRunArgsWithProject(project.Name, svc, svcName, i, project)
					if err := o.driver.RunContainer(ctx, args); err != nil {
						errCh <- fmt.Errorf("starting service %s replica %d: %w", svcName, i, err)
						return
					}

					policy := parseRestartPolicy(svc.Restart)
					if policy != RestartNo {
						go rm.monitorAndRestart(monCtx, project.Name, svcName, policy, args)
					}
				}

				o.logger.Successf("Service %s started (%d replica(s))", svcName, replicas)
			}(serviceName, service)
		}

		wg.Wait()
		close(errCh)

		for err := range errCh {
			monCancel()
			return err
		}
	}

	// 6. Set up service discovery (/etc/hosts entries for service name resolution)
	if err := o.setupServiceDiscovery(ctx, project, opts.Scale); err != nil {
		o.logger.Warnf("Service discovery setup partially failed: %v", err)
	}

	// 7. Apply shm_size by remounting /dev/shm with the correct size
	o.applyShmSize(ctx, project, opts.Scale)

	return nil
}

func (o *Orchestrator) Down(ctx context.Context, project *types.Project, opts DownOptions) error {
	if o.cancelMon != nil {
		o.cancelMon()
	}

	order, err := dependencyOrder(project)
	if err != nil {
		return fmt.Errorf("resolving dependencies: %w", err)
	}

	for i := len(order) - 1; i >= 0; i-- {
		serviceName := order[i]
		service := project.Services[serviceName]

		o.logger.Infof("Stopping service %s", serviceName)

		containers, _ := o.driver.ListContainers(ctx, project.Name)
		stopped := false
		for _, c := range containers {
			if c.Service == serviceName {
				if err := o.driver.StopContainer(ctx, c.Name); err != nil {
					o.logger.Warnf("Failed to stop %s: %v", c.Name, err)
				}
				if err := o.driver.ForceDeleteContainer(ctx, c.Name); err != nil {
					o.logger.Warnf("Failed to remove %s: %v", c.Name, err)
				}
				stopped = true
			}
		}

		// Fallback: try container_name override, then default name
		if !stopped {
			containerName := converter.ContainerName(project.Name, serviceName, 1)
			if service.ContainerName != "" {
				containerName = service.ContainerName
			}
			_ = o.driver.StopContainer(ctx, containerName)
			_ = o.driver.ForceDeleteContainer(ctx, containerName)
			if service.ContainerName != "" {
				genName := converter.ContainerName(project.Name, serviceName, 1)
				_ = o.driver.StopContainer(ctx, genName)
				_ = o.driver.ForceDeleteContainer(ctx, genName)
			}
		}
	}

	defaultNet := converter.NetworkName(project.Name, "default")
	deletedNets := make(map[string]bool)
	for name := range project.Networks {
		networkName := converter.NetworkName(project.Name, name)
		if err := o.driver.DeleteNetwork(ctx, networkName); err != nil {
			o.logger.Warnf("Failed to remove network %s: %v", networkName, err)
		}
		deletedNets[networkName] = true
	}
	// Always try to remove the default network (avoid double-delete)
	if !deletedNets[defaultNet] {
		if err := o.driver.DeleteNetwork(ctx, defaultNet); err != nil {
			o.logger.Warnf("Failed to remove network %s: %v", defaultNet, err)
		}
	}

	if opts.RemoveOrphans {
		containers, _ := o.driver.ListContainers(ctx, project.Name)
		for _, c := range containers {
			if _, exists := project.Services[c.Service]; !exists {
				o.logger.Infof("Removing orphan container %s", c.Name)
				_ = o.driver.StopContainer(ctx, c.Name)
				_ = o.driver.DeleteContainer(ctx, c.Name)
			}
		}
	}

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

func (o *Orchestrator) findService(serviceName string) *types.ServiceConfig {
	if o.project == nil {
		return nil
	}
	if svc, ok := o.project.Services[serviceName]; ok {
		return &svc
	}
	return nil
}

// Returns empty string for Docker Hub images (no explicit registry).
func extractRegistry(image string) string {
	ref := image
	if i := strings.LastIndex(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		// Make sure this is a tag, not a port number
		afterColon := ref[i+1:]
		if !strings.Contains(afterColon, "/") {
			ref = ref[:i]
		}
	}

	// If no slash, it's a Docker Hub official image (e.g., "nginx")
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) < 2 {
		return ""
	}

	// If first part contains a dot or colon, it's a registry
	if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
		return parts[0]
	}

	// Otherwise it's a Docker Hub user/image (e.g., "library/nginx")
	return ""
}

// applyShmSize remounts /dev/shm with the requested size for services that
// specify shm_size. Apple Container doesn't support --shm-size natively,
// so we remount after the container starts.
func (o *Orchestrator) applyShmSize(ctx context.Context, project *types.Project, scaleMap map[string]int) {
	for serviceName, service := range project.Services {
		if service.ShmSize <= 0 {
			continue
		}

		shmSizeMB := max(service.ShmSize/(1024*1024), 1)

		replicas := replicaCount(serviceName, service, scaleMap)
		for i := 1; i <= replicas; i++ {
			containerName := converter.ContainerName(project.Name, serviceName, i)
			if service.ContainerName != "" {
				containerName = service.ContainerName
			}

			cmd := fmt.Sprintf("mount -t tmpfs -o size=%dm tmpfs /dev/shm", shmSizeMB)
			_, err := o.driver.ExecSimple(ctx, containerName, []string{"sh", "-c", cmd})
			if err != nil {
				o.logger.Warnf("Cannot set shm_size for %s: %v", containerName, err)
			} else {
				o.logger.Debugf("Set /dev/shm to %dMB for %s", shmSizeMB, containerName)
			}
		}
	}
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

		opts := driver.BuildOptions{
			Dockerfile: service.Build.Dockerfile,
			Target:     service.Build.Target,
			NoCache:    service.Build.NoCache,
		}

		if len(service.Build.Args) > 0 {
			opts.Args = make(map[string]*string)
			maps.Copy(opts.Args, service.Build.Args)
		}

		for _, cache := range service.Build.CacheFrom {
			opts.CacheFrom = append(opts.CacheFrom, cache)
		}

		if err := o.driver.BuildImageWithOptions(ctx, contextPath, tag, opts); err != nil {
			return fmt.Errorf("building %s: %w", name, err)
		}
	}
	return nil
}

// setupServiceDiscovery injects /etc/hosts entries into all containers so that
// services can resolve each other by service name (Docker Compose compatibility).
// Also adds host.docker.internal pointing to the network gateway (host IP).
func (o *Orchestrator) setupServiceDiscovery(ctx context.Context, project *types.Project, scaleMap map[string]int) error {
	type hostEntry struct {
		ip            string
		serviceName   string
		container     string
		containerName string // explicit container_name from compose (may differ from container)
		hostname      string // explicit hostname from compose
	}

	var entries []hostEntry
	var containerNames []string
	var gatewayIP string

	for serviceName, service := range project.Services {
		replicas := replicaCount(serviceName, service, scaleMap)
		for i := 1; i <= replicas; i++ {
			containerName := converter.ContainerName(project.Name, serviceName, i)
			if service.ContainerName != "" {
				containerName = service.ContainerName
			}

			ip, err := o.driver.GetContainerIP(ctx, containerName)
			if err != nil {
				o.logger.Warnf("Cannot get IP for %s: %v", containerName, err)
				continue
			}

			// Get gateway IP from the first container
			if gatewayIP == "" {
				if gw, err := o.driver.GetContainerGateway(ctx, containerName); err == nil {
					gatewayIP = gw
				}
			}

			entries = append(entries, hostEntry{
				ip:            ip,
				serviceName:   serviceName,
				container:     converter.ContainerName(project.Name, serviceName, i),
				containerName: service.ContainerName,
				hostname:      service.Hostname,
			})
			containerNames = append(containerNames, containerName)
		}
	}

	if len(entries) == 0 {
		return nil
	}

	var hostsLines []string

	// Add host.docker.internal and gateway.docker.internal -> gateway IP
	if gatewayIP != "" {
		hostsLines = append(hostsLines,
			fmt.Sprintf("%s host.docker.internal gateway.docker.internal", gatewayIP))
	}

	for _, e := range entries {
		names := e.serviceName
		if e.container != e.serviceName {
			names += " " + e.container
		}
		if e.containerName != "" && e.containerName != e.serviceName && e.containerName != e.container {
			names += " " + e.containerName
		}
		// Add hostname alias (Docker Compose hostname: field)
		if e.hostname != "" && e.hostname != e.serviceName && e.hostname != e.container && e.hostname != e.containerName {
			names += " " + e.hostname
		}
		hostsLines = append(hostsLines, fmt.Sprintf("%s %s", e.ip, names))
	}
	hostsContent := strings.Join(hostsLines, "\n")

	// Build a set of read-only containers (need special handling for /etc/hosts)
	readOnlyContainers := make(map[string]bool)
	for serviceName, service := range project.Services {
		if service.ReadOnly {
			cn := converter.ContainerName(project.Name, serviceName, 1)
			if service.ContainerName != "" {
				cn = service.ContainerName
			}
			readOnlyContainers[cn] = true
		}
	}

	// Inject into each container via shell (append to /etc/hosts)
	o.logger.Infof("Setting up service discovery for %d containers", len(containerNames))
	for _, containerName := range containerNames {
		if readOnlyContainers[containerName] {
			// Read-only rootfs: make /etc writable via tmpfs overlay
			makeEtcWritable := "cp -a /etc /dev/shm/etc.bak && mount -t tmpfs tmpfs /etc && cp -a /dev/shm/etc.bak/. /etc/ && rm -rf /dev/shm/etc.bak"
			_, err := o.driver.ExecSimple(ctx, containerName, []string{"sh", "-c", makeEtcWritable})
			if err != nil {
				o.logger.Warnf("Cannot make /etc writable in %s: %v", containerName, err)
				continue
			}
		}

		shellCmd := fmt.Sprintf("echo '%s' >> /etc/hosts", hostsContent)
		_, err := o.driver.ExecSimple(ctx, containerName, []string{"sh", "-c", shellCmd})
		if err != nil {
			_, err = o.driver.ExecSimple(ctx, containerName, []string{"/bin/sh", "-c", shellCmd})
			if err != nil {
				o.logger.Warnf("Cannot inject hosts into %s: %v", containerName, err)
			}
		}
	}

	// Set /etc/hostname for containers with explicit hostname (Docker compat)
	for serviceName, service := range project.Services {
		if service.Hostname == "" {
			continue
		}
		containerName := converter.ContainerName(project.Name, serviceName, 1)
		if service.ContainerName != "" {
			containerName = service.ContainerName
		}
		shellCmd := fmt.Sprintf("echo '%s' > /etc/hostname && hostname '%s' 2>/dev/null || true", service.Hostname, service.Hostname)
		_, _ = o.driver.ExecSimple(ctx, containerName, []string{"sh", "-c", shellCmd})
	}

	if gatewayIP != "" {
		o.logger.Successf("Service discovery configured (%d services, host.docker.internal=%s)", len(entries), gatewayIP)
	} else {
		o.logger.Successf("Service discovery configured (%d services)", len(entries))
	}
	return nil
}
