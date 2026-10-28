<div align="center">
  <img src="ui/public/favicon.png" alt="Dockyard" width="128" />

  # Dockyard

  Local-first Docker environment manager for Linux. A single static binary with a k9s-style TUI (default) and an optional embedded Vue web UI.

</div>

## Features

- **TUI** - Bubble Tea terminal UI with compose grouping, live stats sparklines, logs, inspect, start/stop/restart/remove
- **Web UI** - Vue 3 dashboard with live WebSocket updates, container detail graphs (uPlot), resource management
- **Compose-aware** - groups containers by `com.docker.compose.project` label
- **Live stats** - background poller with ring-buffer cache for sparklines and graphs
- **Event stream** - Docker events for real-time state changes
- **Restart-loop detection** - flags containers restarting repeatedly
- **Log shipping** - follows container stdout and stderr and pushes them to GoLogs or Loki

Replaces Portainer/Dockge for homelab use - talks directly to `/var/run/docker.sock` via the official Docker SDK.

## Quick start

### Installation

Install from [Releases](https://github.com/jdbnet/dockyard/releases) or our APT repo...

```bash
curl -fsSL https://apt.jdbnet.co.uk/install/stable.sh | sudo bash
sudo apt install dockyard
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
updates:
  interval: 1h
  autoupdate: false
  autoupdate_interval: ""
compose:
  stacks_dir: /opt/stacks
```

The same values can be edited under Settings in the web UI. Saving writes them back to `config.yaml`. Login and **auto-update** settings apply immediately. The Docker socket, web address, stacks directory, and **update scan interval** apply after a restart.

Environment overrides: `DOCKYARD_DOCKER_SOCKET`, `DOCKYARD_WEB_PORT`, `DOCKYARD_WEB_BIND`, `DOCKYARD_WEB_ENABLED`, `DOCKYARD_AUTH_USER`, `DOCKYARD_AUTH_PASS`, `DOCKYARD_STACKS_DIR`, `DOCKYARD_UPDATES_INTERVAL`. These win over the file when Dockyard starts.

### Web authentication

Set credentials in `config.yaml` (or via env). When `auth.username` is empty, the web UI is open without login.

```yaml
auth:
  username: ""
  password: ""
```

Leave `username` blank to disable auth (default for local-only use). Set both username and password to enable login.

Other apps should use an API key instead of this login. Create one under Settings → API keys. See [docs/api.md](docs/api.md).

### Logging

Dockyard can follow container logs and push them to [GoLogs](https://github.com/jdbnet/gologs) or Grafana Loki. Both accept the same JSON body on `POST /loki/api/v1/push`. The shipper runs inside the Dockyard process. Settings are stored in `config.yaml` next to the other options. Passwords and tokens are kept in that file, like `auth.password`, and the web UI masks them after they are saved.

```yaml
logging:
  enabled: false
  url: http://127.0.0.1:8080/loki/api/v1/push
  type: gologs   # gologs or loki
  auth: none     # none, basic, or bearer
  username: ""
  password: ""
  token: ""
  collect_by_default: true
```

Open Settings, Logging to set the destination, choose none, basic, or bearer authentication, and use Test connection. That sends a single line, `dockyard logging test`. A 401 or 403 from the destination pauses shipping until you save settings again. The same page shows the last successful push, error count, buffered bytes, and any containers that cannot be collected.

Unsent batches are kept in a `logging` directory beside the config file (for `/etc/dockyard/config.yaml`, that is `/etc/dockyard/logging`). The buffer is capped at 32 MiB. Batches go out when they reach 1 MiB or after 2 seconds.

Collection:

- `collect_by_default: true` ships every running container unless it is labelled `dockyard.logs=false`.
- `collect_by_default: false` ships only containers labelled `dockyard.logs=true`.
- Only the `json-file` and `local` log drivers can be read. Other drivers are listed as not collectable.
- Containers that are already running when shipping starts are followed from now. Containers that start afterwards are followed from the beginning. A saved cursor is used after Dockyard restarts so lines are not duplicated or skipped.
- If Dockyard itself is running in Docker, its own container is skipped unless that container has `dockyard.logs=true`.

Extra labels on a stream come from Docker labels with the prefix `dockyard.logs.label.`. For example, `dockyard.logs.label.env=prod` becomes the Loki label `env=prod`. Each stream also has `host`, `container`, `image`, `stream` (`stdout` or `stderr`), and, for Compose, `compose_project` and `compose_service`.

### Auto-update

Dockyard compares local image digests to the registry (HEAD request) on a schedule. You can apply updates manually from the **Updates** page, or enable automatic updates.

Global settings in `config.yaml` (or **Settings → General**):

```yaml
updates:
  interval: 1h              # scan interval (restart required to change)
  autoupdate: false         # apply updates when a newer image is found
  autoupdate_interval: ""   # optional check interval; empty uses interval above
```

**Auto-update all containers** and **Auto-update check interval** apply **without a restart**. The scan interval still needs a restart.

Precedence:

- `dockyard.autoupdate=true` opts a container in; `false` opts it out. These beat the global toggle.
- `dockyard.interval` (for example `1m`, minimum `10s`) checks that container more often. When auto-update is off, Dockyard still reports pending updates sooner but does not apply them.
- If neither label is set, the global `autoupdate` setting applies.
- **Safety rules always apply:** Dockyard never auto-updates its own container; images without a registry digest (locally built or loaded) are never auto-updated through the global setting; explicit `dockyard.autoupdate=true` on such an image shows a failure instead.

When auto-update applies an update, the previous image is removed automatically if nothing else uses it.

### Labels

Docker labels are fixed when a container is created. Recreate the container after changing labels.

| Label | Values | Default | Effect |
| --- | --- | --- | --- |
| `dockyard.autoupdate` | `true` / `false` | (follow global) | Opt in or out of automatic updates |
| `dockyard.interval` | Go duration (`30s`, `1m`, …) | scan interval | How often to check this container (minimum `10s`) |
| `dockyard.logs` | `true` / `false` | `collect_by_default` | Include or exclude log shipping |
| `dockyard.logs.label.<name>` | any string | - | Extra log stream label `<name>` |
| `com.docker.compose.project` | project name | - | Compose grouping and stack updates |
| `com.docker.compose.service` | service name | - | Compose service for `docker compose pull` |
| `com.docker.compose.project.working_dir` | path | - | External compose project directory |

Example:

```yaml
services:
  store:
    image: ghcr.io/<owner>/<image>:latest
    labels:
      dockyard.autoupdate: "true"
      dockyard.interval: "1m"
```

Private **ghcr.io** images need `docker login ghcr.io` (token with `read:packages`) as the user Dockyard runs as. For the systemd unit that is usually root (`/root/.docker/config.json`). For the Docker image, mount `config.json` as shown in `docker-compose.yaml`.

Only **tag** references are checked (`nginx:latest`), not `@sha256` pins.

With global auto-update on, every eligible container on `:latest` is recreated when a new image appears. Opt important stateful services out with `dockyard.autoupdate=false`.

### Update notifications

Dockyard scans for newer container images on the `updates.interval` schedule (default 1 hour). **Update notifications** are separate: they send webhooks only when there are pending updates.

Configure them under **Settings → Update notifications**. You can add multiple webhooks (Discord, Slack, Microsoft Teams Workflows with an Adaptive Card, or custom). Webhook URLs and bearer tokens are stored in `config.yaml` but are not shown again in the API after save.

**Message templates** (Discord, Slack, Teams) use placeholders such as `{{count}}`, `{{summary}}`, `{{names}}`, and `{{link}}`. Leave blank for the default layout. Discord and Teams also support an optional **title template**. `{{summary}}` uses real newline characters between containers.

**Custom** webhooks let you define any JSON keys you need (for example `title`, `message`, and `category` for a homelab dashboard). Each value is a template string; Dockyard builds the JSON body and escapes special characters automatically. For HTML dashboards, `{{summary}}` uses `<br>` between lines; use `{{summary_plain}}` if you need `\n` in the JSON string instead.

**Schedule modes:**

- **Instant** - notify when the set of pending updates changes (hash of container id + remote digest).
- **Interval** - repeat while updates remain (for example every 3h, 6h, or 12h).
- **Daily** - once per day at a set local time while updates remain.

Optional **public URL** adds an “Open updates” link to formatted messages (`{public_url}/updates`).

Example custom field mapping:

| Key | Value template |
|-----|----------------|
| `title` | `{{count}} image updates` |
| `message` | `{{summary}}` |
| `category` | `dockyard` |

Use **Test** on a webhook to send a sample payload without waiting for a real update.

## TUI keybindings

| Key | Action |
|-----|--------|
| `:` | Command bar (`:containers`, `:compose`, `:updates`, `:images`, `:volumes`, `:networks`) |
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

[MIT](LICENSE)
