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
		"--hostname web",                          // DNS: defaults to service name
		"--network myproject_default",             // default network when none specified
	}

	for _, check := range checks {
		if !strings.Contains(argsStr, check) {
			t.Errorf("expected args to contain %q, got: %s", check, argsStr)
		}
	}
}

func TestContainerRunArgsEntrypoint(t *testing.T) {
	service := types.ServiceConfig{
		Image:      "myimage",
		Entrypoint: types.ShellCommand{"/bin/sh", "-c", "echo hello"},
	}

	args := ContainerRunArgs("proj", service, "svc", 1)
	argsStr := strings.Join(args, " ")

	// Only the executable should be passed to --entrypoint
	if !strings.Contains(argsStr, "--entrypoint /bin/sh") {
		t.Errorf("expected --entrypoint /bin/sh, got: %s", argsStr)
	}
	// Should NOT contain the joined form
	if strings.Contains(argsStr, "--entrypoint /bin/sh -c echo hello") {
		t.Errorf("entrypoint should only be the executable, got: %s", argsStr)
	}
}

func TestContainerRunArgsExplicitNetwork(t *testing.T) {
	service := types.ServiceConfig{
		Image: "nginx",
		Networks: map[string]*types.ServiceNetworkConfig{
			"frontend": nil,
		},
	}

	args := ContainerRunArgs("proj", service, "web", 1)
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "--network proj_frontend") {
		t.Errorf("expected --network proj_frontend, got: %s", argsStr)
	}
	// Should NOT contain the default network
	if strings.Contains(argsStr, "proj_default") {
		t.Errorf("should not include default network when explicit network set, got: %s", argsStr)
	}
}

func TestContainerRunArgsCustomHostname(t *testing.T) {
	service := types.ServiceConfig{
		Image:    "nginx",
		Hostname: "myhost",
	}

	args := ContainerRunArgs("proj", service, "web", 1)
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "--hostname myhost") {
		t.Errorf("expected custom hostname, got: %s", argsStr)
	}
}

func TestExtraNetworks(t *testing.T) {
	service := types.ServiceConfig{
		Networks: map[string]*types.ServiceNetworkConfig{
			"frontend": nil,
			"backend":  nil,
		},
	}

	extras := ExtraNetworks("proj", service)
	if len(extras) != 1 {
		t.Errorf("expected 1 extra network, got %d", len(extras))
	}
}

func TestDNSDiscovery(t *testing.T) {
	service := types.ServiceConfig{
		Image: "postgres",
	}

	args := ContainerRunArgs("myapp", service, "db", 1)
	argsStr := strings.Join(args, " ")

	// Should set hostname to service name for DNS discovery
	if !strings.Contains(argsStr, "--hostname db") {
		t.Errorf("expected --hostname db for DNS discovery, got: %s", argsStr)
	}

	// Should set DNS domain to project name
	if !strings.Contains(argsStr, "--dns-domain myapp") {
		t.Errorf("expected --dns-domain myapp, got: %s", argsStr)
	}

	// Should be on default network
	if !strings.Contains(argsStr, "--network myapp_default") {
		t.Errorf("expected --network myapp_default, got: %s", argsStr)
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
