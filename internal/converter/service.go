package converter

import (
	"fmt"
	"strings"

	"github.com/compose-spec/compose-go/v2/types"
)

// ContainerRunArgs converts a compose ServiceConfig into arguments for `container run`.
func ContainerRunArgs(projectName string, service types.ServiceConfig, serviceName string, replica int) []string {
	return ContainerRunArgsWithProject(projectName, service, serviceName, replica, nil)
}

// ContainerRunArgsWithProject converts a compose ServiceConfig into arguments for `container run`,
// with project-level secrets and configs resolution.
func ContainerRunArgsWithProject(projectName string, service types.ServiceConfig, serviceName string, replica int, project *types.Project) []string {
	containerName := ContainerName(projectName, serviceName, replica)

	args := []string{"run", "--name", containerName, "-d"}

	// Labels for compose metadata
	args = appendLabel(args, "com.docker.compose.project", projectName)
	args = appendLabel(args, "com.docker.compose.service", serviceName)
	args = appendLabel(args, "com.docker.compose.container-number", fmt.Sprintf("%d", replica))

	// User-defined labels
	for k, v := range service.Labels {
		args = appendLabel(args, k, v)
	}

	// Environment variables
	for k, v := range service.Environment {
		if v != nil {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, *v))
		} else {
			args = append(args, "-e", k)
		}
	}

	// Env files
	for _, envFile := range service.EnvFiles {
		args = append(args, "--env-file", envFile.Path)
	}

	// Port mappings
	for _, port := range service.Ports {
		portStr := formatPort(port)
		args = append(args, "-p", portStr)
	}

	// Expose (document internal ports, no host binding)
	for _, expose := range service.Expose {
		args = append(args, "--expose", expose)
	}

	// Volumes / bind mounts
	for _, vol := range service.Volumes {
		volStr := formatVolume(projectName, vol)
		args = append(args, "-v", volStr)
	}

	// Tmpfs
	for _, tmpfs := range service.Tmpfs {
		args = append(args, "--tmpfs", tmpfs)
	}

	// Networks: only one can be attached at run time; extras need post-create connect
	// We store the full list so the driver can handle multi-network attachment.
	if len(service.Networks) > 0 {
		// Pick the first network for `run --network`
		for network := range service.Networks {
			args = append(args, "--network", NetworkName(projectName, network))
			break
		}
	} else {
		// Services without explicit networks join the project default network
		args = append(args, "--network", NetworkName(projectName, "default"))
	}

	// Working directory
	if service.WorkingDir != "" {
		args = append(args, "-w", service.WorkingDir)
	}

	// User
	if service.User != "" {
		args = append(args, "-u", service.User)
	}

	// Additional groups
	for _, group := range service.GroupAdd {
		args = append(args, "--group-add", group)
	}

	// Entrypoint: only the executable; extra args become part of the command
	if len(service.Entrypoint) > 0 {
		args = append(args, "--entrypoint", service.Entrypoint[0])
	}

	// CPU limit
	if service.CPUS > 0 {
		args = append(args, "-c", fmt.Sprintf("%.0f", service.CPUS))
	}

	// Memory limit
	if service.MemLimit > 0 {
		args = append(args, "-m", fmt.Sprintf("%d", service.MemLimit))
	}

	// DNS
	for _, dns := range service.DNS {
		args = append(args, "--dns", dns)
	}

	// DNS search domain: add project name so services can resolve by short name
	if len(service.DNSSearch) > 0 {
		for _, search := range service.DNSSearch {
			args = append(args, "--dns-search", search)
		}
	}

	// DNS options
	for _, opt := range service.DNSOpts {
		args = append(args, "--dns-option", opt)
	}

	// DNS domain: set to project name for service discovery
	args = append(args, "--dns-domain", projectName)

	// Init
	if service.Init != nil && *service.Init {
		args = append(args, "--init")
	}

	// Read-only rootfs
	if service.ReadOnly {
		args = append(args, "--read-only")
	}

	// Platform
	if service.Platform != "" {
		args = append(args, "--platform", service.Platform)
	}

	// Hostname: default to service name for DNS discovery between services
	hostname := service.Hostname
	if hostname == "" {
		hostname = serviceName
	}
	args = append(args, "--hostname", hostname)

	// Container name override
	if service.ContainerName != "" {
		// Override the auto-generated name
		args[2] = service.ContainerName
	}

	// TTY
	if service.Tty {
		args = append(args, "-t")
	}

	// Stdin open
	if service.StdinOpen {
		args = append(args, "-i")
	}

	// Stop signal
	if service.StopSignal != "" {
		args = append(args, "--stop-signal", service.StopSignal)
	}

	// Stop grace period
	if service.StopGracePeriod != nil {
		duration := service.StopGracePeriod.String()
		args = append(args, "--stop-timeout", duration)
	}

	// Extra hosts
	for host, ips := range service.ExtraHosts {
		for _, ip := range ips {
			args = append(args, "--add-host", host+":"+ip)
		}
	}

	// Domainname
	if service.DomainName != "" {
		args = append(args, "--domainname", service.DomainName)
	}

	// MAC address
	if service.MacAddress != "" {
		args = append(args, "--mac-address", service.MacAddress)
	}

	// SHM size
	if service.ShmSize > 0 {
		args = append(args, "--shm-size", fmt.Sprintf("%d", service.ShmSize))
	}

	// Annotations
	for k, v := range service.Annotations {
		args = append(args, "--annotation", fmt.Sprintf("%s=%s", k, v))
	}

	// Deploy resource limits (cpu/memory)
	if service.Deploy != nil && service.Deploy.Resources.Limits != nil {
		res := service.Deploy.Resources.Limits
		if res.NanoCPUs > 0 {
			args = append(args, "-c", fmt.Sprintf("%.0f", float32(res.NanoCPUs)))
		}
		if res.MemoryBytes > 0 {
			args = append(args, "-m", fmt.Sprintf("%d", res.MemoryBytes))
		}
	}

	// Secrets: mount as read-only bind mounts at /run/secrets/<name>
	for _, secret := range service.Secrets {
		target := secret.Target
		if target == "" {
			target = "/run/secrets/" + secret.Source
		}
		if project != nil {
			if secretDef, ok := project.Secrets[secret.Source]; ok && secretDef.File != "" {
				args = append(args, "-v", secretDef.File+":"+target+":ro")
			}
		}
	}

	// Configs: mount as read-only bind mounts at /<name>
	for _, config := range service.Configs {
		target := config.Target
		if target == "" {
			target = "/" + config.Source
		}
		if project != nil {
			if configDef, ok := project.Configs[config.Source]; ok && configDef.File != "" {
				args = append(args, "-v", configDef.File+":"+target+":ro")
			}
		}
	}

	// Logging driver
	if service.Logging != nil && service.Logging.Driver != "" {
		args = append(args, "--log-driver", service.Logging.Driver)
		for k, v := range service.Logging.Options {
			args = append(args, "--log-opt", fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Pull policy (used as a hint)
	if service.PullPolicy != "" {
		args = append(args, "--pull", service.PullPolicy)
	}

	// Healthcheck: pass definition as container flags if available
	if service.HealthCheck != nil && !service.HealthCheck.Disable {
		if len(service.HealthCheck.Test) > 0 {
			// Test can be ["CMD", "arg1", ...] or ["CMD-SHELL", "cmd"]
			// Pass the full test as a healthcheck-cmd
			test := service.HealthCheck.Test
			if len(test) > 1 && (test[0] == "CMD" || test[0] == "CMD-SHELL") {
				args = append(args, "--health-cmd", strings.Join(test[1:], " "))
			} else {
				args = append(args, "--health-cmd", strings.Join(test, " "))
			}
		}
		if service.HealthCheck.Interval != nil {
			args = append(args, "--health-interval", service.HealthCheck.Interval.String())
		}
		if service.HealthCheck.Timeout != nil {
			args = append(args, "--health-timeout", service.HealthCheck.Timeout.String())
		}
		if service.HealthCheck.Retries != nil {
			args = append(args, "--health-retries", fmt.Sprintf("%d", *service.HealthCheck.Retries))
		}
		if service.HealthCheck.StartPeriod != nil {
			args = append(args, "--health-start-period", service.HealthCheck.StartPeriod.String())
		}
	}

	// Links: legacy service linking via DNS aliases.
	// Format: "service" or "service:alias". We add --add-host entries so the
	// alias resolves to the linked service's container hostname.
	for _, link := range service.Links {
		parts := strings.SplitN(link, ":", 2)
		linkedService := parts[0]
		alias := linkedService
		if len(parts) == 2 {
			alias = parts[1]
		}
		// The linked service's hostname equals its service name (set via --hostname).
		// We add an alias pointing to the linked service name so DNS resolves it.
		args = append(args, "--add-host", alias+":"+linkedService)
	}

	// Image (required, always last before command)
	args = append(args, service.Image)

	// Command
	if len(service.Command) > 0 {
		args = append(args, service.Command...)
	}

	return args
}

// ContainerName returns the container name for a service replica.
func ContainerName(projectName, serviceName string, replica int) string {
	return fmt.Sprintf("%s-%s-%d", projectName, serviceName, replica)
}

// NetworkName returns the full network name for a project network.
func NetworkName(projectName, network string) string {
	return fmt.Sprintf("%s_%s", projectName, network)
}

// VolumeName returns the full volume name for a project volume.
func VolumeName(projectName, volume string) string {
	return fmt.Sprintf("%s_%s", projectName, volume)
}

func appendLabel(args []string, key, value string) []string {
	return append(args, "-l", fmt.Sprintf("%s=%s", key, value))
}

func formatPort(port types.ServicePortConfig) string {
	var parts []string

	if port.HostIP != "" {
		parts = append(parts, port.HostIP+":")
	}

	if port.Published != "" {
		parts = append(parts, port.Published+":")
	}

	parts = append(parts, fmt.Sprintf("%d", port.Target))

	if port.Protocol != "" && port.Protocol != "tcp" {
		parts = append(parts, "/"+port.Protocol)
	}

	return strings.Join(parts, "")
}

func formatVolume(projectName string, vol types.ServiceVolumeConfig) string {
	source := vol.Source
	// Named volumes get project prefix
	if vol.Type == "volume" && source != "" {
		source = VolumeName(projectName, source)
	}

	result := source + ":" + vol.Target

	if vol.ReadOnly {
		result += ":ro"
	}

	return result
}

// ExtraNetworks returns the networks beyond the first one that need post-create attachment.
func ExtraNetworks(projectName string, service types.ServiceConfig) []string {
	if len(service.Networks) <= 1 {
		return nil
	}

	first := true
	var extras []string
	for network := range service.Networks {
		if first {
			first = false
			continue
		}
		extras = append(extras, NetworkName(projectName, network))
	}
	return extras
}
