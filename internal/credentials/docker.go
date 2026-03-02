// Package credentials reads Docker's credential store to sync registry auth with Apple Container.
package credentials

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type DockerConfig struct {
	Auths      map[string]AuthEntry `json:"auths"`
	CredsStore string               `json:"credsStore"`
}

type AuthEntry struct {
	Auth string `json:"auth,omitempty"`
}

type Credential struct {
	ServerURL string `json:"ServerURL"`
	Username  string `json:"Username"`
	Secret    string `json:"Secret"`
}

func LoadDockerConfig() (*DockerConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	path := filepath.Join(home, ".docker", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read Docker config: %w", err)
	}

	var config DockerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("cannot parse Docker config: %w", err)
	}
	return &config, nil
}

// It reads ~/.docker/config.json to find the credsStore, then calls
// docker-credential-<store> get to retrieve the actual credentials.
func GetCredential(registry string) (*Credential, error) {
	config, err := LoadDockerConfig()
	if err != nil {
		return nil, err
	}

	if config.Auths == nil {
		return nil, fmt.Errorf("no auths in Docker config")
	}

	found := false
	for server := range config.Auths {
		if server == registry || server == "https://"+registry || server == "http://"+registry {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("no Docker credentials for %s", registry)
	}

	if config.CredsStore == "" {
		return nil, fmt.Errorf("no credential helper configured in Docker config")
	}

	return getFromHelper(config.CredsStore, registry)
}

// getFromHelper calls docker-credential-<store> get with the registry on stdin.
func getFromHelper(store, registry string) (*Credential, error) {
	helperName := "docker-credential-" + store
	cmd := exec.Command(helperName, "get")
	cmd.Stdin = strings.NewReader(registry)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s get failed for %s: %w\n%s", helperName, registry, err, stderr.String())
	}

	var cred Credential
	if err := json.Unmarshal(stdout.Bytes(), &cred); err != nil {
		return nil, fmt.Errorf("parsing credential helper output: %w", err)
	}
	return &cred, nil
}

func RegistriesWithCredentials() []string {
	config, err := LoadDockerConfig()
	if err != nil {
		return nil
	}
	var registries []string
	for server := range config.Auths {
		server = strings.TrimPrefix(server, "https://")
		server = strings.TrimPrefix(server, "http://")
		registries = append(registries, server)
	}
	return registries
}
