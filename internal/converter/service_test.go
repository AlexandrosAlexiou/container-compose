package converter

import (
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
)

func TestContainerRunArgs(t *testing.T) {
	env := "world"
	service := types.ServiceConfig{
		Image: "nginx:latest",
		Ports: []types.ServicePortConfig{
			{Target: 80, Published: "8080"},
		},
		Environment: types.MappingWithEquals{
			"HELLO": &env,
		},
		Volumes: []types.ServiceVolumeConfig{
			{Type: "bind", Source: "/host/data", Target: "/data"},
		},
	}

	args := ContainerRunArgs("myproject", service, "web", 1)

	argsStr := strings.Join(args, " ")

	// Check essential args
	checks := []string{
		"run",
		"--name myproject-web-1",
		"-d",
		"nginx:latest",
		"-p 8080:80",
		"-e HELLO=world",
		"-v /host/data:/data",
		"-l com.docker.compose.project=myproject",
		"-l com.docker.compose.service=web",
	}

	for _, check := range checks {
		if !strings.Contains(argsStr, check) {
			t.Errorf("expected args to contain %q, got: %s", check, argsStr)
		}
	}
}

func TestContainerName(t *testing.T) {
	name := ContainerName("myapp", "web", 1)
	if name != "myapp-web-1" {
		t.Errorf("expected myapp-web-1, got %s", name)
	}
}

func TestNetworkName(t *testing.T) {
	name := NetworkName("myapp", "frontend")
	if name != "myapp_frontend" {
		t.Errorf("expected myapp_frontend, got %s", name)
	}
}

func TestVolumeName(t *testing.T) {
	name := VolumeName("myapp", "db-data")
	if name != "myapp_db-data" {
		t.Errorf("expected myapp_db-data, got %s", name)
	}
}

func TestFormatVolumeNamed(t *testing.T) {
	vol := types.ServiceVolumeConfig{
		Type:   "volume",
		Source: "dbdata",
		Target: "/var/lib/data",
	}

	result := formatVolume("myapp", vol)
	if result != "myapp_dbdata:/var/lib/data" {
		t.Errorf("expected myapp_dbdata:/var/lib/data, got %s", result)
	}
}

func TestFormatVolumeReadOnly(t *testing.T) {
	vol := types.ServiceVolumeConfig{
		Type:     "bind",
		Source:   "/host",
		Target:   "/container",
		ReadOnly: true,
	}

	result := formatVolume("myapp", vol)
	if result != "/host:/container:ro" {
		t.Errorf("expected /host:/container:ro, got %s", result)
	}
}
