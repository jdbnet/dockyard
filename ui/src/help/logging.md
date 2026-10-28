# Logging

Ship container stdout/stderr to **GoLogs** or **Loki** from **Settings → Logging**.

- `collect_by_default: true` — ship all running containers unless `dockyard.logs=false`.
- `collect_by_default: false` — ship only `dockyard.logs=true`.

Supported drivers: **json-file** and **local** only.

Dockyard skips its own container unless `dockyard.logs=true`. Extra stream labels come from `dockyard.logs.label.<name>`.

Use **Test connection** to send a single test line.
