# Settings and API

## Settings page

Maps to `config.yaml` next to the binary (often `/etc/dockyard/config.yaml` under systemd).

**Apply immediately:** web login, restart-loop threshold, **auto-update** toggle and interval, logging destination, notification webhooks.

**Apply after restart:** Docker socket, web bind/port, stacks directory, stats/events/TUI intervals, **update scan interval**.

## Web auth

Optional username/password for the UI. Leave username blank for no login on trusted networks.

## API keys and external API

Create keys under **Settings → API keys**. External apps call `/api/v1/external/*` with `Authorization: Bearer …`.

Interactive OpenAPI docs: [/docs](/docs) (Swagger UI). Human-readable notes: [docs/api.md](https://github.com/jdbnet/dockyard/blob/main/docs/api.md) in the repository.

Session API (used by the web UI) includes `GET` / `PUT /api/v1/settings` with `updates_autoupdate` and `updates_autoupdate_interval` alongside the other general fields.
