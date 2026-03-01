// Package orchestrator manages the lifecycle of a compose project, including dependency ordering,
// service discovery, health checks, and restart policies.
package orchestrator

import (
	"fmt"

	"github.com/compose-spec/compose-go/v2/types"
)

// checkPortConflicts validates that no two services (or replicas) claim the same host port.
func checkPortConflicts(project *types.Project, scale map[string]int) error {
	type portClaim struct {
		service string
		port    string
	}

	hostPorts := make(map[string]portClaim) // "hostIP:hostPort/protocol" -> claim

	for name, service := range project.Services {
		replicas := replicaCount(name, service, scale)

		for _, port := range service.Ports {
			if port.Published == "" {
				continue
			}

			protocol := port.Protocol
			if protocol == "" {
				protocol = "tcp"
			}

			hostIP := port.HostIP
			if hostIP == "" {
				hostIP = "0.0.0.0"
			}

			key := fmt.Sprintf("%s:%s/%s", hostIP, port.Published, protocol)

			if replicas > 1 {
				return fmt.Errorf(
					"service %q publishes host port %s but is scaled to %d replicas; "+
						"host ports cannot be shared across replicas — remove the port mapping or use a single replica",
					name, port.Published, replicas,
				)
			}

			if existing, ok := hostPorts[key]; ok {
				return fmt.Errorf(
					"port conflict: host port %s is claimed by both service %q and %q",
					key, existing.service, name,
				)
			}
			hostPorts[key] = portClaim{service: name, port: port.Published}
		}
	}

	return nil
}

// replicaCount returns how many replicas to run for a service.
// Priority: --scale flag > deploy.replicas > 1.
func replicaCount(serviceName string, service types.ServiceConfig, scaleOverride map[string]int) int {
	if n, ok := scaleOverride[serviceName]; ok {
		return n
	}

	if service.Deploy != nil && service.Deploy.Replicas != nil {
		return *service.Deploy.Replicas
	}

	return 1
}
