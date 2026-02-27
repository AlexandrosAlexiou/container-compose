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

# Use a specific compose file
container-compose -f my-compose.yml up -d

# Specify project name
container-compose -p myapp up -d
```

## Supported Compose Features

| Feature | Status |
|---------|--------|
| `image` | ✅ |
| `build` | ✅ |
| `ports` | ✅ |
| `volumes` (bind + named) | ✅ |
| `environment` | ✅ |
| `env_file` | ✅ |
| `networks` | ✅ |
| `command` | ✅ |
| `entrypoint` | ✅ |
| `working_dir` | ✅ |
| `user` | ✅ |
| `depends_on` (ordering) | ✅ |
| `depends_on` (service_healthy) | ✅ |
| `cpus` / `mem_limit` | ✅ |
| `dns` / `dns_search` | ✅ |
| `init` | ✅ |
| `read_only` | ✅ |
| `tmpfs` | ✅ |
| `labels` | ✅ |
| `platform` | ✅ |
| `hostname` | ✅ |
| `profiles` | ✅ |
| `extends` / `includes` | ✅ (via compose-go) |
| `restart` policies | ✅ |
| `deploy.replicas` | ✅ |
| DNS service discovery | ✅ (via hostname + shared network) |
| Port conflict detection | ✅ |
| `secrets` / `configs` | 🚧 Planned |
| Multiple networks per service | ⚠️ First network only |
| `privileged` / `cap_add` | ❌ N/A (VM isolation) |
| `devices` | ❌ N/A |
| `network_mode: host` | ❌ N/A (separate VMs) |

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
    image: node:20-alpine
    working_dir: /app
    command: ["node", "server.js"]
    volumes:
      - ./api:/app
    environment:
      DATABASE_URL: postgres://db:5432/myapp
    depends_on:
      - db

  db:
    image: postgres:16
    environment:
      POSTGRES_DB: myapp
      POSTGRES_PASSWORD: secret
    volumes:
      - db-data:/var/lib/postgresql/data

volumes:
  db-data:
```

```bash
container-compose up -d
container-compose ps
container-compose logs -f api
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
1. Sets each container's `--hostname` to its service name
2. Places all services without explicit networks on the project's default network
3. Apple Container's built-in DNS resolves hostnames within the same network

## Architecture

```
container-compose (Go)
  ├── compose-go library (parses docker-compose.yml)
  ├── orchestrator (dependency ordering, lifecycle)
  ├── converter (ServiceConfig → CLI args)
  └── driver (executes `container` CLI commands)
        └── container CLI (Apple's Swift binary)
```

## Debugging

Set `COMPOSE_DEBUG=1` to see the `container` CLI commands being executed:

```bash
COMPOSE_DEBUG=1 container-compose up
```

## License

Apache License 2.0
