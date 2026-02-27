package orchestrator

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
)

func TestDependencyOrderNoDeps(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"web":   {Image: "nginx"},
			"db":    {Image: "postgres"},
			"cache": {Image: "redis"},
		},
	}

	order, err := dependencyOrder(project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("expected 3 services, got %d", len(order))
	}

	// All services should be present
	seen := make(map[string]bool)
	for _, s := range order {
		seen[s] = true
	}
	for name := range project.Services {
		if !seen[name] {
			t.Errorf("service %q missing from order", name)
		}
	}
}

func TestDependencyOrderWithDeps(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"web": {
				Image:     "nginx",
				DependsOn: map[string]types.ServiceDependency{"api": {Condition: "service_started"}},
			},
			"api": {
				Image:     "myapi",
				DependsOn: map[string]types.ServiceDependency{"db": {Condition: "service_started"}},
			},
			"db": {
				Image: "postgres",
			},
		},
	}

	order, err := dependencyOrder(project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// db must come before api, api must come before web
	indexOf := make(map[string]int)
	for i, s := range order {
		indexOf[s] = i
	}

	if indexOf["db"] >= indexOf["api"] {
		t.Errorf("db (index %d) should come before api (index %d)", indexOf["db"], indexOf["api"])
	}
	if indexOf["api"] >= indexOf["web"] {
		t.Errorf("api (index %d) should come before web (index %d)", indexOf["api"], indexOf["web"])
	}
}

func TestDependencyOrderCircular(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"a": {
				Image:     "img",
				DependsOn: map[string]types.ServiceDependency{"b": {Condition: "service_started"}},
			},
			"b": {
				Image:     "img",
				DependsOn: map[string]types.ServiceDependency{"a": {Condition: "service_started"}},
			},
		},
	}

	_, err := dependencyOrder(project)
	if err == nil {
		t.Fatal("expected error for circular dependency")
	}
}
