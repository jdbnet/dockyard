# Labels

Labels are set at **container create** time. Edit compose or `docker run`, then recreate the container.

| Label | Values | Effect |
| --- | --- | --- |
| `dockyard.autoupdate` | `true` / `false` | Opt in or out of auto-update (overrides global) |
| `dockyard.interval` | e.g. `1m`, `30s` (min `10s`) | Check interval for this container |
| `dockyard.logs` | `true` / `false` | Force collect or skip log shipping |
| `dockyard.logs.label.*` | string | Extra log label (suffix becomes key) |
| `com.docker.compose.project` | name | Compose project |
| `com.docker.compose.service` | name | Compose service |
| `com.docker.compose.project.working_dir` | path | External compose directory |

## Auto-update example

Global **auto-update on**, but opt out one database:

```yaml
services:
  api:
    image: ghcr.io/example/api:latest
    labels:
      dockyard.autoupdate: "true"
  db:
    image: postgres:16
    labels:
      dockyard.autoupdate: "false"
```

```bash
docker run -d --label dockyard.interval=1m nginx:latest
```
