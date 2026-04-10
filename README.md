# container-compose

Docker Compose compatible orchestration for [Apple Container](https://github.com/apple/container).

`container-compose` reads standard `docker-compose.yml` files and orchestrates multi-container applications using Apple's native container runtime on macOS.

## Requirements

- macOS 15 (Sequoia) or later on Apple Silicon
- [Apple Container](https://github.com/apple/container) CLI installed
- Go 1.21+ (for building from source)

## Installation

```bash
make build
# Binary will be at bin/container-compose

# Or install to /usr/local/bin:
sudo make install
```

## Commands

| Command | Description |
|---------|-------------|
| `up` | Create and start containers (`-d`, `--build`, `--scale`) |
| `down` | Stop and remove containers, networks (`-v`, `--remove-orphans`) |
| `ps` | List running services |
| `logs` | View output from containers (`-f`, `--tail`) |
| `build` | Build or rebuild services |
| `exec` | Execute a command in a running service container (`-d`, `-u`, `-w`, `-T`) |
| `run` | Run a one-off command on a service (`--rm`, `-e`, `-u`, `-w`) |
| `start` | Start existing stopped containers |
| `stop` | Stop running containers without removing |
| `restart` | Restart service containers |
| `create` | Create containers without starting them |
| `rm` | Remove stopped service containers (`-f`) |
| `kill` | Send a signal to service containers (`-s`) |
| `pull` | Pull service images |
| `push` | Push service images |
| `cp` | Copy files to/from service containers |
| `top` | Display running processes |
| `port` | Print the public port for a port binding |
| `images` | List images used by services |
| `rmi` | Remove images used by services |
| `stats` | Display container resource usage |
| `config` | Validate and render compose file |
| `version` | Show version information |
| `wait` | Block until a service container stops |
| `attach` | Attach stdin/stdout/stderr to a running container |
| `login` | Log in to a container registry |
| `logout` | Log out from a container registry |
| `network ls` | List networks |
| `network create` | Create a network |
| `network rm` | Remove one or more networks |
| `network prune` | Remove all unused project networks (`-f`) |

## Usage

```bash
# Start all services (foreground)
container-compose up

# Start in background
container-compose up -d

# Build images before starting
container-compose up --build

# Scale a service to multiple replicas
container-compose up --scale worker=3

# Stop and remove all services
container-compose down

# Remove volumes on down
container-compose down -v

# List running services
container-compose ps

# View logs
container-compose logs
container-compose logs -f web

# Build images
container-compose build
container-compose build api

# Execute a command in a running container (interactive by default)
container-compose exec db psql -U postgres

# Execute with flags passed to the container command
container-compose exec kafka kafka-console-producer --topic my-topic --bootstrap-server kafka:9092

# Execute non-interactively (e.g. for scripting)
container-compose exec -T db psql -U postgres -c "SELECT 1"

# Run a one-off command
container-compose run web python manage.py migrate

# Lifecycle management
container-compose stop web
container-compose start web
container-compose restart web
container-compose kill -s SIGTERM web

# Image operations
container-compose pull
container-compose push
container-compose images

# Remove images used by services
container-compose rmi
container-compose rmi web api

# Copy files
container-compose cp web:/app/logs ./logs
container-compose cp ./config.yml web:/app/config.yml

# Inspect
container-compose top
container-compose stats
container-compose port web 80

# Network management
container-compose network ls
container-compose network create mynetwork
container-compose network rm mynetwork
container-compose network prune --force

# Validate compose file
container-compose config

# Use a specific compose file
container-compose -f my-compose.yml up -d

# Specify project name
container-compose -p myapp up -d
```

### Private Registries

**With Docker installed:** `container-compose` automatically reads Docker's credential store (`~/.docker/config.json`). If you've already logged in with `docker login`, `az acr login`, or any Docker credential helper, those credentials are synced to Apple Container on `up`:

```bash
# Any of these work — no extra step needed
az acr login --name myregistry
# or
docker login ghcr.io

# container-compose picks up the credentials automatically
container-compose up -d
```

**Without Docker:** Log in directly using Apple Container's registry store:

```bash
container-compose login myregistry.azurecr.io
# or with explicit credentials
container-compose login -u myuser myregistry.azurecr.io
```

If credentials are missing for a private registry, `container-compose` will warn you with the exact command to run.

## Supported Compose Features

### Service Configuration

| Feature | Status |
|---------|--------|
| `image` | ✓ |
| `build` (context, dockerfile, args, target, cache_from, no_cache) | ✓ |
| `ports` | ✓ |
| `volumes` (bind + named) | ✓ |
| `environment` | ✓ |
| `env_file` | ✓ |
| `networks` | ✓ |
| `command` | ✓ |
| `entrypoint` | ✓ |
| `working_dir` | ✓ |
| `user` | ✓ |
| `container_name` | ✓ |
| `depends_on` (ordering) | ✓ |
| `depends_on` (service_healthy) | ✓ |
| `cpus` / `mem_limit` | ✓ |
| `deploy.replicas` | ✓ |
| `deploy.resources` (limits) | ✓ |
| `dns` / `dns_search` | ✓ |
| `extra_hosts` | ✓ |
| `hostname` / `domainname` | ✓ |
| `init` | ✓ |
| `read_only` | ✓ |
| `tmpfs` | ✓ |
| `tty` / `stdin_open` | ✓ |
| `stop_signal` / `stop_grace_period` | ✓ |
| `labels` / `annotations` | ✓ |
| `platform` | ✓ |
| `pull_policy` | ✓ |
| `logging` (driver + options) | ✓ |
| `mac_address` | ✓ |
| `shm_size` | ✓ |
| `healthcheck` (test, interval, timeout, retries) | ✓ |
| `links` (DNS aliases) | ✓ |
| `expose` | ✓ |
| `secrets` (file-based) | ✓ |
| `configs` (file-based) | ✓ |
| `profiles` | ✓ |
| `extends` / `includes` | ✓ (via compose-go) |
| `restart` policies | ✓ |
| DNS service discovery | ✓ (via hostname + shared network) |
| Port conflict detection | ✓ |

### How Features Map to Apple Container

Some Docker Compose features don't map directly to Apple Container CLI flags. `container-compose` uses workarounds to bridge the gap:

| Compose Feature | How It's Mapped |
|---|---|
| `hostname` | Injected as alias in `/etc/hosts` + `/etc/hostname` set after start |
| `shm_size` | `/dev/shm` remounted with correct size via `mount -t tmpfs -o size=Xm` |
| `host.docker.internal` | Gateway IP injected into `/etc/hosts` automatically |
| `container_name` | Used as container ID + DNS alias in `/etc/hosts` |
| `read_only` + service discovery | `/etc` overlaid with tmpfs to allow `/etc/hosts` writes |
| Anonymous volumes (`- /var/run`) | Mapped to `--tmpfs` mounts |
| `stop_signal` / `stop_grace_period` | Stored as labels (Apple Container uses fixed SIGTERM) |
| `healthcheck` | Exec'd inside container after start (not native health support) |
| Service discovery | `/etc/hosts` injection (not DNS-based like Docker) |
| `links` | Stored as labels for metadata |

### Limitations

#### Not Possible (VM Architecture)

Apple Container runs each container in a separate lightweight VM — not a shared-kernel namespace like Docker. This means 22 Docker Compose features are architecturally impossible:

| Feature | Why |
|---|---|
| `privileged` / `cap_add` / `cap_drop` | Each VM has its own full kernel — no capabilities to add or drop |
| `devices` / `gpus` | No host device/GPU passthrough to VMs |
| `network_mode: host` | Each VM has its own network stack |
| `pid` / `ipc` namespace sharing | VMs have separate kernels; can't share PID/IPC across them |
| `volumes_from` | Separate VMs can't share mount namespaces |
| `security_opt` / `sysctls` | VM isolation replaces seccomp/apparmor; sysctls are per-VM kernel |
| `userns_mode` / `uts` | Each VM has its own user and UTS namespace |
| `cgroup_parent` / `pids_limit` | VMs aren't in host cgroups |
| `oom_kill_disable` / `storage_opt` | VM memory/storage managed differently |
| `pause` / `unpause` | Apple Container doesn't support pause |
| `runtime` | Single runtime (Apple VM) — no runc/kata/etc. |
| `credential_spec` / `isolation` | Windows-only features |

> **Impact:** Most real-world compose files don't use these features. If your compose file does use them, those lines will be silently ignored — the containers will still run, just without the specific kernel-level tuning.

#### Not Yet Implemented

These 4 features are possible but require significant work:

| Feature | Description |
|---|---|
| `events` | Real-time event stream (`container-compose events`) — requires event streaming from Apple Container CLI |
| `watch` / `develop` | File sync + auto-rebuild on source changes — requires file watcher and container restart logic |
| Recreate changed only | On `up`, only recreate containers whose config changed — requires config diffing and state tracking |

#### Known Quirks

| Issue | Workaround |
|---|---|
| **XPC timeouts** | Apple Container occasionally times out stopping containers via XPC, leaving ghost references. `container-compose up` force-removes existing containers before starting to handle this. |
| **Service discovery is /etc/hosts-based** | Unlike Docker's embedded DNS, we inject entries into `/etc/hosts`. This means: no wildcard DNS, no round-robin for scaled services, and entries are static (set at startup). |
| **shm_size is applied after start** | The `/dev/shm` remount happens after the container starts. If a process checks `/dev/shm` size during very early init, it may see the default 64MB briefly. |
| **Private registries** | Credentials from `docker login` or `az acr login` are automatically synced — no extra login step needed. You can also use `container-compose login <registry>` directly. |

## Example

```yaml
# docker-compose.yml
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
    volumes:
      - ./html:/usr/share/nginx/html:ro
    depends_on:
      - api

  api:
    build:
      context: ./api
      args:
        NODE_ENV: production
      target: runtime
    working_dir: /app
    command: ["node", "server.js"]
    environment:
      DATABASE_URL: postgres://db:5432/myapp
    secrets:
      - db_password
    depends_on:
      - db

  db:
    image: postgres:16
    environment:
      POSTGRES_DB: myapp
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    volumes:
      - db-data:/var/lib/postgresql/data
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 512M

secrets:
  db_password:
    file: ./secrets/db_password.txt

volumes:
  db-data:
```

```bash
container-compose up -d
container-compose ps
container-compose logs -f api
container-compose exec db psql -U postgres
container-compose down -v
```

## Service Discovery

Services on the same network automatically resolve each other by service name. In this example, the `api` service can connect to `db` using hostname `db`:

```yaml
services:
  api:
    image: myapi
    environment:
      DATABASE_URL: postgres://db:5432/myapp
    depends_on:
      - db

  db:
    image: postgres:16
```

This works because `container-compose`:
1. Inspects each container's IP address after startup
2. Injects `/etc/hosts` entries into every container mapping service names to IPs
3. Places all services without explicit networks on the project's default network

The following DNS names are automatically resolvable from inside any container:

| Name | Resolves To |
|---|---|
| `<service-name>` (e.g. `db`) | Container IP of that service |
| `<container-name>` (e.g. `myproject-db-1`) | Same container IP |
| `<container_name>` override | If `container_name:` is set in compose |
| `<hostname>` override | If `hostname:` is set in compose |
| `host.docker.internal` | Host machine IP (gateway) |
| `gateway.docker.internal` | Same as above |

This means `WORDPRESS_DB_HOST: db`, `DATABASE_URL: postgres://db:5432/myapp`, and `${DOCKER_GATEWAY_HOST:-host.docker.internal}` patterns all work out of the box — just like Docker Compose.

## Testing

```bash
# Unit tests (no runtime needed)
make test

# Integration tests (requires running Apple Container runtime)
make test-integration

# Integration tests, skip heavy images (WordPress)
make test-integration-short
```

Integration test fixtures are in `testdata/fixtures/` and cover:
- Single service, multi-service with depends_on
- Environment variables, env_file, named/bind/anonymous volumes
- Service discovery (hostname resolution between containers)
- `host.docker.internal` and `gateway.docker.internal` resolution
- `hostname` alias injection and `/etc/hostname` setting
- `shm_size` remounting (verified with `df`)
- `read_only` rootfs with writable tmpfs mounts
- `container_name` overrides and DNS aliases
- `user`, `ulimits`, `command`
- `depends_on: condition: service_healthy` (exec-based healthchecks)
- `depends_on: condition: service_started`
- Full WordPress + MySQL stack (real-world compose file)
- Comprehensive `TestFullFeatures` test validating 14 features together

## Architecture

`container-compose` uses [`compose-spec/compose-go`](https://github.com/compose-spec/compose-go) (v2) as its compose file parser. This is the **official Go reference implementation** of the [Compose specification](https://compose-spec.io/) — the same library used by Docker Compose itself.

### What compose-go provides

By depending on compose-go, `container-compose` gets full spec compliance without reimplementing the parser:

- **File discovery** — automatically finds `docker-compose.yml`, `compose.yml`, or `compose.yaml` in the working directory, and respects the `COMPOSE_FILE` environment variable
- **YAML parsing and schema validation** — parses compose files and validates them against the Compose specification JSON schema
- **Variable interpolation** — resolves `${VAR}`, `${VAR:-default}`, and `${VAR:?error}` patterns using the process environment and `.env` files
- **Extends and includes** — resolves `extends` and `include` directives across multiple files, with full merge semantics
- **Profile filtering** — filters services based on active `--profile` flags
- **Type normalization** — normalizes the many YAML representations (e.g., ports as strings or objects, durations as strings or integers) into consistent Go structs
- **Dependency graph** — resolves `depends_on` references and validates that referenced services exist

In short: **compose-go handles "what does the user want?"** and **container-compose handles "how do we make Apple Container do it?"**

## Debugging

Set `COMPOSE_DEBUG=1` to see the `container` CLI commands being executed:

```bash
COMPOSE_DEBUG=1 container-compose up
```

## Apple Container Extensions (`x-apple-container`)

Apple Container has unique capabilities that Docker doesn't offer. Standard compose files work unchanged, but you can opt in to Apple-specific features using the `x-apple-container` extension field. Docker Compose and other tools will silently ignore these fields — your compose file stays portable.

### Available Extensions

```yaml
services:
  my-service:
    image: myapp:latest
    x-apple-container:
      rosetta: true
      ssh: true
      virtualization: true
      no-dns: true
      init-image: "my-init:latest"
      publish-socket:
        - "/run/app.sock:/var/run/app.sock"
```

| Extension | Apple Container Flag | Description |
|---|---|---|
| `rosetta: true` | `--rosetta` | Enable Rosetta translation for running x86_64 Linux binaries on Apple Silicon. Use this when your image is amd64-only and you don't want to rebuild for arm64. |
| `ssh: true` | `--ssh` | Forward the host's SSH agent socket into the container. Enables `git clone` over SSH, private dependency installation, and key-based auth inside the container without copying keys. |
| `virtualization: true` | `--virtualization` | Expose virtualization capabilities to the container. Enables running nested VMs inside the container (requires host and guest support). |
| `no-dns: true` | `--no-dns` | Do not configure DNS in the container. Useful for containers that manage their own `/etc/resolv.conf`. |
| `init-image: "<image>"` | `--init-image <image>` | Use a custom init image instead of Apple Container's default. The init process runs as PID 1 and reaps zombie processes. |
| `publish-socket` | `--publish-socket` | Publish Unix domain sockets from the container to the host. Format: `container_path:host_path`. Unlike TCP port forwarding, this uses Unix sockets for lower latency IPC. |

### Use Cases

#### Cross-Architecture Development

Run x86_64 Linux images on Apple Silicon without rebuilding:

```yaml
services:
  legacy-api:
    image: mycompany/legacy-api:latest  # amd64 only
    platform: linux/amd64
    x-apple-container:
      rosetta: true  # Rosetta translates x86_64 → arm64
    ports:
      - "8080:8080"
```

#### Git Operations Inside Containers

Forward your SSH agent for private repo access during builds or at runtime:

```yaml
services:
  builder:
    image: node:20
    x-apple-container:
      ssh: true  # SSH agent forwarded
    command: ["sh", "-c", "git clone git@github.com:myorg/private-repo.git && npm install"]
    volumes:
      - .:/workspace
```

#### CI/CD with Nested Virtualization

Run VMs inside containers for testing infrastructure tools:

```yaml
services:
  test-runner:
    image: ubuntu:24.04
    x-apple-container:
      virtualization: true  # Nested VM support
    command: ["sh", "-c", "qemu-system-aarch64 ..."]
```

#### Unix Socket Communication

Low-latency IPC between host and container using Unix sockets:

```yaml
services:
  database:
    image: postgres:16
    x-apple-container:
      publish-socket:
        - "/var/run/postgresql/.s.PGSQL.5432:/tmp/pg.sock"
    environment:
      POSTGRES_PASSWORD: secret
```

### Portability

These extensions are designed to be safely ignored:

| Tool | Behavior |
|---|---|
| `container-compose` | ✓ Extensions applied |
| `docker compose` | ✓ `x-` fields silently ignored |
| `podman-compose` | ✓ `x-` fields silently ignored |
| Any compose-spec tool | ✓ `x-` fields are part of the spec |

This means you can use one compose file for both Docker and Apple Container. Developers on Apple Silicon get the extra features; everyone else gets standard Docker behavior.

## License

Apache License 2.0
