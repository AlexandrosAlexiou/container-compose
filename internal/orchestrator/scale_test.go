package orchestrator

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
)

func TestReplicaCountDefault(t *testing.T) {
	service := types.ServiceConfig{Image: "nginx"}
	n := replicaCount("web", service, nil)
	if n != 1 {
		t.Errorf("expected 1, got %d", n)
	}
}

func TestReplicaCountDeploy(t *testing.T) {
	replicas := 3
	service := types.ServiceConfig{
		Image:  "nginx",
		Deploy: &types.DeployConfig{Replicas: &replicas},
	}
	n := replicaCount("web", service, nil)
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

func TestReplicaCountScaleOverride(t *testing.T) {
	replicas := 2
	service := types.ServiceConfig{
		Image:  "nginx",
		Deploy: &types.DeployConfig{Replicas: &replicas},
	}
	n := replicaCount("web", service, map[string]int{"web": 5})
	if n != 5 {
		t.Errorf("expected 5 (scale override), got %d", n)
	}
}

func TestCheckPortConflictsNone(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"web": {
				Image: "nginx",
				Ports: []types.ServicePortConfig{{Target: 80, Published: "8080"}},
			},
			"api": {
				Image: "node",
				Ports: []types.ServicePortConfig{{Target: 3000, Published: "3000"}},
			},
		},
	}

	if err := checkPortConflicts(project, nil); err != nil {
		t.Errorf("expected no conflict, got: %v", err)
	}
}

func TestCheckPortConflictsBetweenServices(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"web": {
				Image: "nginx",
				Ports: []types.ServicePortConfig{{Target: 80, Published: "8080"}},
			},
			"api": {
				Image: "node",
				Ports: []types.ServicePortConfig{{Target: 3000, Published: "8080"}},
			},
		},
	}

	err := checkPortConflicts(project, nil)
	if err == nil {
		t.Error("expected port conflict error")
	}
}

func TestCheckPortConflictsScaledWithPorts(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"web": {
				Image: "nginx",
				Ports: []types.ServicePortConfig{{Target: 80, Published: "8080"}},
			},
		},
	}

	err := checkPortConflicts(project, map[string]int{"web": 3})
	if err == nil {
		t.Error("expected error: scaled service with host ports")
	}
}

func TestCheckPortConflictsScaledWithoutPorts(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"worker": {
				Image: "worker",
			},
		},
	}

	if err := checkPortConflicts(project, map[string]int{"worker": 5}); err != nil {
		t.Errorf("expected no conflict for portless scaled service, got: %v", err)
	}
}
