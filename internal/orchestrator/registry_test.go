package orchestrator

import "testing"

func TestExtractRegistry(t *testing.T) {
	tests := []struct {
		image    string
		expected string
	}{
		// Docker Hub official images — no registry
		{"nginx", ""},
		{"nginx:latest", ""},
		{"nginx:1.25", ""},

		// Docker Hub user images — no registry
		{"library/nginx", ""},
		{"myuser/myapp", ""},
		{"myuser/myapp:v1.0", ""},

		// Private registries
		{"myregistry.azurecr.io/myapp", "myregistry.azurecr.io"},
		{"myregistry.azurecr.io/myapp:latest", "myregistry.azurecr.io"},
		{"ghcr.io/owner/repo:sha-abc123", "ghcr.io"},
		{"registry.example.com/team/app:v2", "registry.example.com"},
		{"quay.io/coreos/etcd:v3.5", "quay.io"},

		// Registry with port
		{"localhost:5000/myimage", "localhost:5000"},
		{"localhost:5000/myimage:tag", "localhost:5000"},

		// With digest
		{"myregistry.azurecr.io/myapp@sha256:abc123", "myregistry.azurecr.io"},

		// AWS ECR
		{"123456789.dkr.ecr.us-east-1.amazonaws.com/myapp:latest", "123456789.dkr.ecr.us-east-1.amazonaws.com"},

		// Deep paths
		{"myregistry.azurecr.io/team/project/app:v1", "myregistry.azurecr.io"},

		// Empty string
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			got := extractRegistry(tt.image)
			if got != tt.expected {
				t.Errorf("extractRegistry(%q) = %q, want %q", tt.image, got, tt.expected)
			}
		})
	}
}

func TestExtractRegistryDeduplication(t *testing.T) {
	// Test that multiple images from the same registry yield the same result
	images := []string{
		"myregistry.azurecr.io/app1:latest",
		"myregistry.azurecr.io/app2:v2",
		"myregistry.azurecr.io/team/app3:latest",
	}

	registries := make(map[string]bool)
	for _, img := range images {
		r := extractRegistry(img)
		if r != "" {
			registries[r] = true
		}
	}

	if len(registries) != 1 {
		t.Errorf("expected 1 unique registry, got %d: %v", len(registries), registries)
	}
	if !registries["myregistry.azurecr.io"] {
		t.Errorf("expected myregistry.azurecr.io, got %v", registries)
	}
}
