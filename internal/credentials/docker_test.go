package credentials

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setupTempDockerConfig creates a temporary ~/.docker/config.json and overrides HOME.
// Returns a cleanup function.
func setupTempDockerConfig(t *testing.T, config DockerConfig) func() {
	t.Helper()
	tmpDir := t.TempDir()
	dockerDir := filepath.Join(tmpDir, ".docker")
	if err := os.MkdirAll(dockerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(dockerDir, "config.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	return func() { os.Setenv("HOME", origHome) }
}

func TestLoadDockerConfig(t *testing.T) {
	cleanup := setupTempDockerConfig(t, DockerConfig{
		Auths: map[string]AuthEntry{
			"myregistry.azurecr.io": {},
			"ghcr.io":              {},
		},
		CredsStore: "osxkeychain",
	})
	defer cleanup()

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

func TestLoadDockerConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	dockerDir := filepath.Join(tmpDir, ".docker")
	os.MkdirAll(dockerDir, 0o755)
	os.WriteFile(filepath.Join(dockerDir, "config.json"), []byte("{invalid}"), 0o644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	_, err := LoadDockerConfig()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRegistriesWithCredentials(t *testing.T) {
	cleanup := setupTempDockerConfig(t, DockerConfig{
		Auths: map[string]AuthEntry{
			"registry.example.com":  {},
			"https://ghcr.io":       {},
			"myregistry.azurecr.io": {},
		},
		CredsStore: "osxkeychain",
	})
	defer cleanup()

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

func TestRegistriesWithCredentialsNoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	registries := RegistriesWithCredentials()
	if len(registries) != 0 {
		t.Errorf("expected 0 registries when no config, got %d", len(registries))
	}
}

func TestGetCredentialNotInAuths(t *testing.T) {
	cleanup := setupTempDockerConfig(t, DockerConfig{
		Auths: map[string]AuthEntry{
			"ghcr.io": {},
		},
		CredsStore: "osxkeychain",
	})
	defer cleanup()

	_, err := GetCredential("unknown-registry.example.com")
	if err == nil {
		t.Error("expected error for registry not in auths")
	}
}

func TestGetCredentialNoAuths(t *testing.T) {
	cleanup := setupTempDockerConfig(t, DockerConfig{
		CredsStore: "osxkeychain",
	})
	defer cleanup()

	_, err := GetCredential("ghcr.io")
	if err == nil {
		t.Error("expected error when auths is nil")
	}
}

func TestGetCredentialNoCredsStore(t *testing.T) {
	cleanup := setupTempDockerConfig(t, DockerConfig{
		Auths: map[string]AuthEntry{
			"ghcr.io": {},
		},
		// No CredsStore
	})
	defer cleanup()

	_, err := GetCredential("ghcr.io")
	if err == nil {
		t.Error("expected error when no credential helper configured")
	}
}

func TestGetCredentialHttpsPrefixMatch(t *testing.T) {
	cleanup := setupTempDockerConfig(t, DockerConfig{
		Auths: map[string]AuthEntry{
			"https://ghcr.io": {},
		},
		CredsStore: "nonexistent-helper-for-test",
	})
	defer cleanup()

	// Should find "ghcr.io" even though config has "https://ghcr.io"
	// Will fail at the helper call (nonexistent), but should NOT fail at the lookup
	_, err := GetCredential("ghcr.io")
	if err == nil {
		t.Error("expected error from nonexistent helper, not from lookup")
	}
	// The error should be about the helper failing, not "no Docker credentials"
	if err != nil && err.Error() == "no Docker credentials for ghcr.io" {
		t.Error("lookup failed to match https://ghcr.io with ghcr.io")
	}
}

func TestGetCredentialHelperNotFound(t *testing.T) {
	cleanup := setupTempDockerConfig(t, DockerConfig{
		Auths: map[string]AuthEntry{
			"ghcr.io": {},
		},
		CredsStore: "nonexistent-helper-xyz",
	})
	defer cleanup()

	_, err := GetCredential("ghcr.io")
	if err == nil {
		t.Error("expected error when credential helper binary not found")
	}
}
