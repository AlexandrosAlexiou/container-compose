package credentials

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDockerConfig(t *testing.T) {
	// Create a temporary Docker config
	tmpDir := t.TempDir()
	dockerDir := filepath.Join(tmpDir, ".docker")
	if err := os.MkdirAll(dockerDir, 0o755); err != nil {
		t.Fatal(err)
	}

	config := DockerConfig{
		Auths: map[string]AuthEntry{
			"myregistry.azurecr.io": {},
			"ghcr.io":              {},
		},
		CredsStore: "osxkeychain",
	}

	data, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(dockerDir, "config.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Override HOME to use our temp dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	loaded, err := LoadDockerConfig()
	if err != nil {
		t.Fatalf("LoadDockerConfig failed: %v", err)
	}

	if loaded.CredsStore != "osxkeychain" {
		t.Errorf("expected credsStore=osxkeychain, got %s", loaded.CredsStore)
	}
	if len(loaded.Auths) != 2 {
		t.Errorf("expected 2 auths, got %d", len(loaded.Auths))
	}
	if _, ok := loaded.Auths["myregistry.azurecr.io"]; !ok {
		t.Error("expected myregistry.azurecr.io in auths")
	}
}

func TestLoadDockerConfigMissing(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	_, err := LoadDockerConfig()
	if err == nil {
		t.Error("expected error for missing Docker config")
	}
}

func TestRegistriesWithCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	dockerDir := filepath.Join(tmpDir, ".docker")
	os.MkdirAll(dockerDir, 0o755)

	config := DockerConfig{
		Auths: map[string]AuthEntry{
			"registry.example.com":     {},
			"https://ghcr.io":          {},
			"myregistry.azurecr.io":    {},
		},
		CredsStore: "osxkeychain",
	}

	data, _ := json.Marshal(config)
	os.WriteFile(filepath.Join(dockerDir, "config.json"), data, 0o644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	registries := RegistriesWithCredentials()
	if len(registries) != 3 {
		t.Errorf("expected 3 registries, got %d: %v", len(registries), registries)
	}

	// Check that https:// prefix is stripped
	found := false
	for _, r := range registries {
		if r == "ghcr.io" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ghcr.io (stripped prefix), got %v", registries)
	}
}
