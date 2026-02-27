package orchestrator

import (
	"fmt"

	"github.com/compose-spec/compose-go/v2/types"
)

// dependencyOrder returns services in topological order based on depends_on.
func dependencyOrder(project *types.Project) ([]string, error) {
	// Build adjacency list
	deps := make(map[string][]string)
	allServices := make(map[string]bool)

	for name, service := range project.Services {
		allServices[name] = true
		for dep := range service.DependsOn {
			deps[name] = append(deps[name], dep)
		}
	}

	// DFS-based topological sort: dependencies are visited first
	visited := make(map[string]int) // 0=unvisited, 1=in-progress, 2=done
	var order []string

	var visit func(name string) error
	visit = func(name string) error {
		if visited[name] == 2 {
			return nil
		}
		if visited[name] == 1 {
			return fmt.Errorf("circular dependency detected involving service %q", name)
		}

		visited[name] = 1

		// Visit dependencies first
		for _, dep := range deps[name] {
			if !allServices[dep] {
				return fmt.Errorf("service %q depends on %q which is not defined", name, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}

		visited[name] = 2
		order = append(order, name)
		return nil
	}

	for name := range allServices {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// dependencyLevels groups services into parallel execution levels.
// Services within the same level have no dependencies on each other
// and can be started concurrently.
func dependencyLevels(project *types.Project) ([][]string, error) {
	order, err := dependencyOrder(project)
	if err != nil {
		return nil, err
	}

	// Assign each service a level = max(level of deps) + 1
	levels := make(map[string]int)
	for _, name := range order {
		level := 0
		service := project.Services[name]
		for dep := range service.DependsOn {
			if depLevel, ok := levels[dep]; ok && depLevel >= level {
				level = depLevel + 1
			}
		}
		levels[name] = level
	}

	// Group by level
	maxLevel := 0
	for _, l := range levels {
		if l > maxLevel {
			maxLevel = l
		}
	}

	result := make([][]string, maxLevel+1)
	for _, name := range order {
		l := levels[name]
		result[l] = append(result[l], name)
	}

	return result, nil
}
