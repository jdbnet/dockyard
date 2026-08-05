# Dockyard

Local-first Docker environment manager for Linux. A single static binary with a k9s-style TUI (default) and an optional embedded Vue web UI.

Replaces Portainer/Dockge for homelab use - talks directly to `/var/run/docker.sock` via the official Docker SDK.

## Features

- **TUI** - Bubble Tea terminal UI with compose grouping, live stats sparklines, logs, inspect, start/stop/restart/remove
- **Web UI** - Vue 3 dashboard with live WebSocket updates, container detail graphs (uPlot), resource management
- **Compose-aware** - groups containers by `com.docker.compose.project` label
- **Live stats** - background poller with ring-buffer cache for sparklines and graphs
- **Event stream** - Docker events for real-time state changes
- **Restart-loop detection** - flags containers restarting repeatedly

## Quick start

### Build

```bash
# Backend only (TUI works without web assets)
go build -o dockyard ./cmd/dockyard

# Full binary with embedded web UI
cd ui && npm install && npm run build && cd ..
go build -o dockyard ./cmd/dockyard

# Or for ARM64...
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dockyard-arm64 ./cmd/dockyard
```

### Run

```bash
# TUI only (default)
./dockyard

# TUI + web UI on http://127.0.0.1:8080
./dockyard --web

# Headless web-only (for systemd)
./dockyard --headless --web

# Debug: list compose-grouped containers
./dockyard --debug-list
```

### Install (systemd)

```bash
go build -o dockyard ./cmd/dockyard
sudo bash deploy/install.sh
```

## Configuration

`config.yaml`:

```yaml
docker:
  socket: /var/run/docker.sock
web:
  enabled: false
  port: 8080
  bind: 127.0.0.1
stats:
  interval: 5s
  buffer_size: 60
events:
  fallback_refresh: 30s
restart_loop:
  threshold: 3
  window: 5m
```

Environment overrides: `DOCKYARD_DOCKER_SOCKET`, `DOCKYARD_WEB_PORT`, `DOCKYARD_WEB_BIND`, `DOCKYARD_WEB_ENABLED`, `DOCKYARD_AUTH_USER`, `DOCKYARD_AUTH_PASS`.

### Web authentication

Set credentials in `config.yaml` (or via env). When `auth.username` is empty, the web UI is open without login.

```yaml
auth:
  username: admin
  password: changeme
```

Leave `username` blank to disable auth (default for local-only use).

## TUI keybindings

| Key | Action |
|-----|--------|
| `:` | Command bar (`:containers`, `:compose`, `:images`, `:volumes`, `:networks`) |
| `/` | Filter |
| `j`/`k` | Navigate |
| `d` | Inspect |
| `l` | Logs |
| `s`/`S`/`r` | Start / Stop / Restart |
| `x` | Remove (confirm) |
| `c` | Toggle compose grouping |
| `R` | Force refresh |
| `?` | Help |
| `q` | Quit |

## Web UI development

```bash
# Terminal 1 - API
go run ./cmd/dockyard --headless --web

# Terminal 2 - Vite dev server (proxies /api and /ws)
cd ui && npm run dev
```

## Comparison

| Feature | Dockyard | Dockge | Portainer |
|---------|----------|--------|-----------|
| Local socket only | Yes | Yes | Yes (also remote) |
| TUI | Yes | No | No |
| Compose grouping | Yes | Yes | Yes |
| Live stats | Yes | Limited | Yes |
| Single static binary | Yes | No (Node) | No |
| Auth required | Optional | Optional | Yes |
| Kubernetes | No | No | Yes |

## Requirements

- Linux with Docker
- User in `docker` group (or root) for socket access
- Go 1.22+ to build

## License

MIT
