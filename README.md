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
| `down` | Stop and remove containers, networks (`-v`) |
| `ps` | List running services |
| `logs` | View output from containers (`-f`, `--tail`) |
| `build` | Build or rebuild services |
| `exec` | Execute a command in a running service container |
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
| `stats` | Display container resource usage |
| `config` | Validate and render compose file |
| `version` | Show version information |
| `wait` | Block until a service container stops |
| `attach` | Attach stdin/stdout/stderr to a running container |
| `login` | Log in to a container registry |
| `logout` | Log out from a container registry |

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

# Execute a command in a running container
container-compose exec db psql -U postgres

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

# Copy files
container-compose cp web:/app/logs ./logs
container-compose cp ./config.yml web:/app/config.yml

# Inspect
container-compose top
container-compose stats
container-compose port web 80

# Validate compose file
container-compose config

# Use a specific compose file
container-compose -f my-compose.yml up -d

# Specify project name
container-compose -p myapp up -d
```

## Supported Compose Features

### Service Configuration

| Feature | Status |
|---------|--------|
| `image` | ✅ |
| `build` (context, dockerfile, args, target, cache_from, no_cache) | ✅ |
| `ports` | ✅ |
| `volumes` (bind + named) | ✅ |
| `environment` | ✅ |
| `env_file` | ✅ |
| `networks` | ✅ |
| `command` | ✅ |
| `entrypoint` | ✅ |
| `working_dir` | ✅ |
| `user` | ✅ |
| `container_name` | ✅ |
| `depends_on` (ordering) | ✅ |
| `depends_on` (service_healthy) | ✅ |
| `cpus` / `mem_limit` | ✅ |
| `deploy.replicas` | ✅ |
| `deploy.resources` (limits) | ✅ |
| `dns` / `dns_search` | ✅ |
| `extra_hosts` | ✅ |
| `hostname` / `domainname` | ✅ |
| `init` | ✅ |
| `read_only` | ✅ |
| `tmpfs` | ✅ |
| `tty` / `stdin_open` | ✅ |
| `stop_signal` / `stop_grace_period` | ✅ |
| `labels` / `annotations` | ✅ |
| `platform` | ✅ |
| `pull_policy` | ✅ |
| `logging` (driver + options) | ✅ |
| `mac_address` | ✅ |
| `shm_size` | ✅ |
| `healthcheck` (test, interval, timeout, retries) | ✅ |
| `links` (DNS aliases) | ✅ |
| `expose` | ✅ |
| `secrets` (file-based) | ✅ |
| `configs` (file-based) | ✅ |
| `profiles` | ✅ |
| `extends` / `includes` | ✅ (via compose-go) |
| `restart` policies | ✅ |
| DNS service discovery | ✅ (via hostname + shared network) |
| Port conflict detection | ✅ |

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
| **Private registries** | Run `container-compose login <registry>` before using private images. Apple Container uses its own credential store, not Docker's `~/.docker/config.json`. |

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

## Debugging

Set `COMPOSE_DEBUG=1` to see the `container` CLI commands being executed:

```bash
COMPOSE_DEBUG=1 container-compose up
```

## License

Apache License 2.0
