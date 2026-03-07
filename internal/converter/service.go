// Package converter translates Docker Compose service definitions into Apple Container CLI arguments.
package converter

import (
	"fmt"
	"strings"

	"github.com/compose-spec/compose-go/v2/types"
)

func ContainerRunArgs(projectName string, service types.ServiceConfig, serviceName string, replica int) []string {
	return ContainerRunArgsWithProject(projectName, service, serviceName, replica, nil)
}

// ContainerRunArgsWithProject builds the full `container run` argument list,
// emitting only flags actually supported by Apple Container CLI.
func ContainerRunArgsWithProject(projectName string, service types.ServiceConfig, serviceName string, replica int, project *types.Project) []string {
	containerName := ContainerName(projectName, serviceName, replica)

	args := []string{"run", "--name", containerName, "-d"}

	args = appendLabel(args, "com.docker.compose.project", projectName)
	args = appendLabel(args, "com.docker.compose.service", serviceName)
	args = appendLabel(args, "com.docker.compose.container-number", fmt.Sprintf("%d", replica))

	for k, v := range service.Labels {
		args = appendLabel(args, k, v)
	}

	for k, v := range service.Environment {
		if v != nil {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, *v))
		} else {
			args = append(args, "-e", k)
		}
	}

	for _, envFile := range service.EnvFiles {
		args = append(args, "--env-file", envFile.Path)
	}

	for _, port := range service.Ports {
		portStr := formatPort(port)
		args = append(args, "-p", portStr)
	}

	// Store expose ports as a label so ps can display them (Docker Compose compat)
	if len(service.Expose) > 0 {
		args = appendLabel(args, "com.docker.compose.expose", strings.Join(service.Expose, ","))
	}

	for _, vol := range service.Volumes {
		// Anonymous volumes (no source) → tmpfs mount
		if vol.Source == "" {
			args = append(args, "--tmpfs", vol.Target)
			continue
		}
		volStr := formatVolume(projectName, vol)
		args = append(args, "-v", volStr)
	}

	for _, tmpfs := range service.Tmpfs {
		args = append(args, "--tmpfs", tmpfs)
	}

	// MAC address is passed via the --network flag format: name,mac=XX:XX:XX:XX:XX:XX
	networkArg := ""
	if len(service.Networks) > 0 {
		for network := range service.Networks {
			networkArg = NetworkName(projectName, network)
			break
		}
	} else {
		networkArg = NetworkName(projectName, "default")
	}
	if service.MacAddress != "" {
		networkArg += ",mac=" + service.MacAddress
	}
	args = append(args, "--network", networkArg)

	if service.WorkingDir != "" {
		args = append(args, "-w", service.WorkingDir)
	}

	if service.User != "" {
		args = append(args, "-u", service.User)
	}

	// Entrypoint: only the executable; extra args become part of the command
	if len(service.Entrypoint) > 0 {
		args = append(args, "--entrypoint", service.Entrypoint[0])
	}

	if service.CPUS > 0 {
		args = append(args, "-c", fmt.Sprintf("%.0f", service.CPUS))
	}

	if service.MemLimit > 0 {
		args = append(args, "-m", fmt.Sprintf("%d", service.MemLimit))
	}

	for _, dns := range service.DNS {
		args = append(args, "--dns", dns)
	}

	for _, search := range service.DNSSearch {
		args = append(args, "--dns-search", search)
	}

	for _, opt := range service.DNSOpts {
		args = append(args, "--dns-option", opt)
	}

	args = append(args, "--dns-domain", projectName)

	if service.Init != nil && *service.Init {
		args = append(args, "--init")
	}

	if service.ReadOnly {
		args = append(args, "--read-only")
	}

	for name, ulimit := range service.Ulimits {
		if ulimit.Single > 0 {
			args = append(args, "--ulimit", fmt.Sprintf("%s=%d", name, ulimit.Single))
		} else {
			args = append(args, "--ulimit", fmt.Sprintf("%s=%d:%d", name, ulimit.Soft, ulimit.Hard))
		}
	}

	if service.Platform != "" {
		args = append(args, "--platform", service.Platform)
	}

	if service.ContainerName != "" {
		args[2] = service.ContainerName
	}

	if service.Tty {
		args = append(args, "-t")
	}

	if service.StdinOpen {
		args = append(args, "-i")
	}

	if service.Deploy != nil && service.Deploy.Resources.Limits != nil {
		res := service.Deploy.Resources.Limits
		if res.NanoCPUs > 0 && service.CPUS == 0 {
			args = append(args, "-c", fmt.Sprintf("%.0f", float32(res.NanoCPUs)))
		}
		if res.MemoryBytes > 0 && service.MemLimit == 0 {
			args = append(args, "-m", fmt.Sprintf("%d", res.MemoryBytes))
		}
	}

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

	// --- Fields not natively supported by Apple Container CLI ---

	if service.Hostname != "" {
		args = appendLabel(args, "com.docker.compose.hostname", service.Hostname)
	}
	if service.StopSignal != "" {
		args = appendLabel(args, "com.docker.compose.stop-signal", service.StopSignal)
	}
	if service.StopGracePeriod != nil {
		args = appendLabel(args, "com.docker.compose.stop-grace-period", service.StopGracePeriod.String())
	}
	if service.DomainName != "" {
		args = appendLabel(args, "com.docker.compose.domainname", service.DomainName)
	}
	if service.ShmSize > 0 {
		shmSizeMB := service.ShmSize / (1024 * 1024)
		args = appendLabel(args, "com.docker.compose.shm-size", fmt.Sprintf("%dm", shmSizeMB))
	}
	for k, v := range service.Annotations {
		args = appendLabel(args, "com.docker.compose.annotation."+k, v)
	}
	if service.Logging != nil && service.Logging.Driver != "" {
		args = appendLabel(args, "com.docker.compose.log-driver", service.Logging.Driver)
	}
	if service.PullPolicy != "" {
		args = appendLabel(args, "com.docker.compose.pull-policy", service.PullPolicy)
	}

	// Extra hosts: stored as env vars for /etc/hosts workaround
	for host, ips := range service.ExtraHosts {
		for _, ip := range ips {
			args = appendLabel(args, "com.docker.compose.extra-host."+host, ip)
		}
	}

	// Links: stored as labels; DNS handled by shared network + dns-domain
	for _, link := range service.Links {
		parts := strings.SplitN(link, ":", 2)
		linkedService := parts[0]
		alias := linkedService
		if len(parts) == 2 {
			alias = parts[1]
		}
		args = appendLabel(args, "com.docker.compose.link."+alias, linkedService)
	}

	if service.HealthCheck != nil && !service.HealthCheck.Disable {
		if len(service.HealthCheck.Test) > 0 {
			test := service.HealthCheck.Test
			if len(test) > 1 && (test[0] == "CMD" || test[0] == "CMD-SHELL") {
				args = appendLabel(args, "com.docker.compose.healthcheck.cmd", strings.Join(test[1:], " "))
			} else {
				args = appendLabel(args, "com.docker.compose.healthcheck.cmd", strings.Join(test, " "))
			}
		}
		if service.HealthCheck.Interval != nil {
			args = appendLabel(args, "com.docker.compose.healthcheck.interval", service.HealthCheck.Interval.String())
		}
		if service.HealthCheck.Retries != nil {
			args = appendLabel(args, "com.docker.compose.healthcheck.retries", fmt.Sprintf("%d", *service.HealthCheck.Retries))
		}
	}

	args = append(args, service.Image)

	// Entrypoint args after the executable (e.g. "-c" "script...") go after the image
	if len(service.Entrypoint) > 1 {
		args = append(args, service.Entrypoint[1:]...)
	}

	if len(service.Command) > 0 {
		args = append(args, service.Command...)
	}

	return args
}

func ContainerName(projectName, serviceName string, replica int) string {
	return fmt.Sprintf("%s-%s-%d", projectName, serviceName, replica)
}

func NetworkName(projectName, network string) string {
	return fmt.Sprintf("%s_%s", projectName, network)
}

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
	if vol.Type == "volume" && source != "" {
		source = VolumeName(projectName, source)
	}

	result := source + ":" + vol.Target

	if vol.ReadOnly {
		result += ":ro"
	}

	return result
}

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
