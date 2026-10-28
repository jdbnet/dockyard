# Deploying

## Stacks

**Managed stacks** live under `compose.stacks_dir` (default `/opt/stacks`). Create or edit compose files in the web UI or TUI, then **Up**, **Down**, **Update**, or **Delete** the stack.

**External** Compose projects are picked up from running containers using `com.docker.compose.project` and `com.docker.compose.project.working_dir`. Dockyard can update their services without importing them into `stacks_dir`.

## Single containers

Containers outside Compose are listed under **Containers**. Start, stop, restart, remove, shell, and **Update** (pull and recreate with the same config).

## How updates run

- **Compose** services: `docker compose pull <service>` then `up -d <service>` for that service.
- **Other** containers: pull the image, stop, remove, recreate with the same config and networks, start if they were running.
