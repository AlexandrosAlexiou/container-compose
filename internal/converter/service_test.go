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
	// Entrypoint args should appear after the image
	if !strings.Contains(argsStr, "myimage -c echo hello") {
		t.Errorf("expected entrypoint args after image, got: %s", argsStr)
	}
}

func TestContainerRunArgsEntrypointWithCommand(t *testing.T) {
	service := types.ServiceConfig{
		Image:      "postgres:14",
		Entrypoint: types.ShellCommand{"bash", "-c", "docker-entrypoint.sh postgres & tail -f /dev/null"},
		Command:    types.ShellCommand{"--some-flag"},
	}

	args := ContainerRunArgs("proj", service, "db", 1)
	argsStr := strings.Join(args, " ")

	// Executable via --entrypoint
	if !strings.Contains(argsStr, "--entrypoint bash") {
		t.Errorf("expected --entrypoint bash, got: %s", argsStr)
	}
	// Entrypoint args then command args after image
	if !strings.Contains(argsStr, "postgres:14 -c docker-entrypoint.sh postgres & tail -f /dev/null --some-flag") {
		t.Errorf("expected entrypoint args followed by command args after image, got: %s", argsStr)
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

	// Hostname stored as label since container CLI doesn't support --hostname
	if !strings.Contains(argsStr, "com.docker.compose.hostname=myhost") {
		t.Errorf("expected hostname label, got: %s", argsStr)
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

func TestLinksArgs(t *testing.T) {
	service := types.ServiceConfig{
		Image: "myapp:latest",
		Links: []string{"db", "cache:redis"},
	}

	args := ContainerRunArgs("proj", service, "web", 1)
	argsStr := strings.Join(args, " ")

	// Links are stored as labels since --add-host is not supported
	if !strings.Contains(argsStr, "com.docker.compose.link.db=db") {
		t.Errorf("expected link label for db, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "com.docker.compose.link.redis=cache") {
		t.Errorf("expected link label for redis alias, got: %s", argsStr)
	}
}

func TestHealthcheckArgs(t *testing.T) {
	interval := types.Duration(10000000000) // 10s
	timeout := types.Duration(5000000000)   // 5s
	retries := uint64(3)

	service := types.ServiceConfig{
		Image: "myapp:latest",
		HealthCheck: &types.HealthCheckConfig{
			Test:     []string{"CMD", "curl", "-f", "http://localhost/"},
			Interval: &interval,
			Timeout:  &timeout,
			Retries:  &retries,
		},
	}

	args := ContainerRunArgs("proj", service, "web", 1)
	argsStr := strings.Join(args, " ")

	// Healthcheck stored as labels since --health-* not supported by container CLI
	if !strings.Contains(argsStr, "com.docker.compose.healthcheck.cmd=curl -f http://localhost/") {
		t.Errorf("expected healthcheck cmd label, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "com.docker.compose.healthcheck.retries=3") {
		t.Errorf("expected healthcheck retries label, got: %s", argsStr)
	}
}

func TestContainerNameOverride(t *testing.T) {
	service := types.ServiceConfig{
		Image:         "nginx:latest",
		ContainerName: "my-custom-name",
	}

	args := ContainerRunArgs("proj", service, "web", 1)

	// The --name should be overridden
	if args[2] != "my-custom-name" {
		t.Errorf("expected container name my-custom-name, got %s", args[2])
	}
}

func TestExposeArgs(t *testing.T) {
	service := types.ServiceConfig{
		Image:  "myapp:latest",
		Expose: []string{"3000", "8080"},
	}

	args := ContainerRunArgs("proj", service, "web", 1)
	argsStr := strings.Join(args, " ")

	// Expose is informational only; since container CLI doesn't support --expose,
	// the ports are not passed as flags. Verify they don't cause errors.
	if strings.Contains(argsStr, "--expose") {
		t.Errorf("--expose should not be in args (not supported by container CLI), got: %s", argsStr)
	}
}

func TestShmSizeMapping(t *testing.T) {
	service := types.ServiceConfig{
		Image:   "mssql:latest",
		ShmSize: 3 * 1024 * 1024 * 1024, // 3GB in bytes
	}

	args := ContainerRunArgs("proj", service, "db", 1)
	argsStr := strings.Join(args, " ")

	// shm_size stored as label (Apple Container doesn't support --tmpfs size option)
	if !strings.Contains(argsStr, "com.docker.compose.shm-size=3072m") {
		t.Errorf("expected shm-size label with 3072m, got: %s", argsStr)
	}
}

func TestShmSizeSmall(t *testing.T) {
	service := types.ServiceConfig{
		Image:   "app:latest",
		ShmSize: 64 * 1024 * 1024, // 64MB
	}

	args := ContainerRunArgs("proj", service, "app", 1)
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "com.docker.compose.shm-size=64m") {
		t.Errorf("expected shm-size label with 64m, got: %s", argsStr)
	}
}

func TestShmSizeMinimum(t *testing.T) {
	service := types.ServiceConfig{
		Image:   "app:latest",
		ShmSize: 100, // sub-MB
	}

	args := ContainerRunArgs("proj", service, "app", 1)
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "com.docker.compose.shm-size=0m") {
		t.Errorf("expected shm-size label, got: %s", argsStr)
	}
}

func TestShmSizeZero(t *testing.T) {
	service := types.ServiceConfig{
		Image:   "app:latest",
		ShmSize: 0, // no shm_size
	}

	args := ContainerRunArgs("proj", service, "app", 1)
	argsStr := strings.Join(args, " ")

	if strings.Contains(argsStr, "com.docker.compose.shm-size") {
		t.Errorf("should not add shm-size label when shm_size is 0, got: %s", argsStr)
	}
}

func TestHostnameLabel(t *testing.T) {
	service := types.ServiceConfig{
		Image:    "azurite:latest",
		Hostname: "azurite",
	}

	args := ContainerRunArgs("proj", service, "blob", 1)
	argsStr := strings.Join(args, " ")

	// Hostname stored as label for orchestrator to handle
	if !strings.Contains(argsStr, "com.docker.compose.hostname=azurite") {
		t.Errorf("expected hostname label, got: %s", argsStr)
	}
}

func TestAnonymousVolume(t *testing.T) {
	service := types.ServiceConfig{
		Image: "nginx",
		Volumes: []types.ServiceVolumeConfig{
			{Type: "volume", Target: "/var/run"},
			{Type: "volume", Target: "/tmp"},
		},
	}

	args := ContainerRunArgs("proj", service, "web", 1)
	argsStr := strings.Join(args, " ")

	// Anonymous volumes (no source) should become --tmpfs
	if !strings.Contains(argsStr, "--tmpfs /var/run") {
		t.Errorf("expected --tmpfs /var/run for anonymous volume, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "--tmpfs /tmp") {
		t.Errorf("expected --tmpfs /tmp for anonymous volume, got: %s", argsStr)
	}
	// Should NOT produce -v :/var/run
	if strings.Contains(argsStr, "-v :/var/run") {
		t.Errorf("should not produce empty source bind mount, got: %s", argsStr)
	}
}

func TestBindVolume(t *testing.T) {
	service := types.ServiceConfig{
		Image: "postgres",
		Volumes: []types.ServiceVolumeConfig{
			{Type: "bind", Source: "/host/init.sql", Target: "/docker-entrypoint-initdb.d/init.sql"},
		},
	}

	args := ContainerRunArgs("proj", service, "db", 1)
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "-v /host/init.sql:/docker-entrypoint-initdb.d/init.sql") {
		t.Errorf("expected bind mount, got: %s", argsStr)
	}
}

func TestUserFlag(t *testing.T) {
	service := types.ServiceConfig{
		Image: "nginx",
		User:  "1000:1000",
	}

	args := ContainerRunArgs("proj", service, "web", 1)
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "-u 1000:1000") {
		t.Errorf("expected -u 1000:1000, got: %s", argsStr)
	}
}

func TestReadOnly(t *testing.T) {
	service := types.ServiceConfig{
		Image:    "nginx",
		ReadOnly: true,
	}

	args := ContainerRunArgs("proj", service, "web", 1)
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "--read-only") {
		t.Errorf("expected --read-only, got: %s", argsStr)
	}
}

func TestUlimits(t *testing.T) {
	service := types.ServiceConfig{
		Image: "elasticsearch",
		Ulimits: map[string]*types.UlimitsConfig{
			"nofile": {Soft: 65535, Hard: 65535},
		},
	}

	args := ContainerRunArgs("proj", service, "es", 1)
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "--ulimit nofile=65535:65535") {
		t.Errorf("expected --ulimit nofile=65535:65535, got: %s", argsStr)
	}
}

func TestUlimitsSingle(t *testing.T) {
	service := types.ServiceConfig{
		Image: "app",
		Ulimits: map[string]*types.UlimitsConfig{
			"nproc": {Single: 1024},
		},
	}

	args := ContainerRunArgs("proj", service, "app", 1)
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "--ulimit nproc=1024") {
		t.Errorf("expected --ulimit nproc=1024, got: %s", argsStr)
	}
}

func TestEnvFile(t *testing.T) {
	service := types.ServiceConfig{
		Image: "app",
		EnvFiles: []types.EnvFile{
			{Path: "/path/to/env.file"},
		},
	}

	args := ContainerRunArgs("proj", service, "app", 1)
	argsStr := strings.Join(args, " ")

	if !strings.Contains(argsStr, "--env-file /path/to/env.file") {
		t.Errorf("expected --env-file, got: %s", argsStr)
	}
}

func TestCommand(t *testing.T) {
	service := types.ServiceConfig{
		Image:   "app",
		Command: types.ShellCommand{"--config", "/etc/app.conf"},
	}

	args := ContainerRunArgs("proj", service, "app", 1)

	// Command args should be appended after the image
	argsStr := strings.Join(args, " ")
	if !strings.Contains(argsStr, "app --config /etc/app.conf") {
		t.Errorf("expected command args after image, got: %s", argsStr)
	}
}
