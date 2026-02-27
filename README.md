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

# Stop and remove all services
container-compose down

# List running services
container-compose ps

# View logs
container-compose logs
container-compose logs -f web

# Use a specific compose file
container-compose -f my-compose.yml up -d

# Specify project name
container-compose -p myapp up -d

# Remove volumes on down
container-compose down -v
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
| `cpus` / `mem_limit` | ✅ |
| `dns` | ✅ |
| `init` | ✅ |
| `read_only` | ✅ |
| `tmpfs` | ✅ |
| `labels` | ✅ |
| `platform` | ✅ |
| `hostname` | ✅ |
| `profiles` | ✅ |
| `extends` / `includes` | ✅ (via compose-go) |
| `healthcheck` (condition wait) | 🚧 Planned |
| `restart` policies | 🚧 Planned |
| `secrets` / `configs` | 🚧 Planned |
| `deploy.replicas` | 🚧 Planned |
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
