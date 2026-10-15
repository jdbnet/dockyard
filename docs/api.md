# External API

Dockyard exposes a small HTTP API for other apps. It uses API keys, not the web UI login. Create a key under **Settings → API keys**. The full key is shown once. Dockyard stores only a SHA-256 hash and a short prefix.

Send the key on every request:

```http
Authorization: Bearer dy_...
```

`read` can call the GET endpoints. `write` can also start, stop, and restart containers. Each key is limited to 60 requests per minute.

Interactive docs are at `/docs` in the web UI. The OpenAPI document is `GET /api/v1/external/openapi.yaml`.

## Examples

```bash
curl -s -H "Authorization: Bearer $DOCKYARD_KEY" \
  http://127.0.0.1:8080/api/v1/external/stats

curl -s -H "Authorization: Bearer $DOCKYARD_KEY" \
  http://127.0.0.1:8080/api/v1/external/containers

curl -s -H "Authorization: Bearer $DOCKYARD_KEY" \
  http://127.0.0.1:8080/api/v1/external/containers/my-app

curl -s -X POST -H "Authorization: Bearer $DOCKYARD_KEY" \
  http://127.0.0.1:8080/api/v1/external/containers/my-app/start
```

`stats` returns the same overall numbers as the dashboard: running and total containers, combined CPU percent, combined memory bytes, current network in and out in bytes per second, and a count per state.

A container `id` may be the full id, the short id, or the name.

## Errors

```json
{"error":"unauthorized"}
```

| Status | Meaning |
| --- | --- |
| 401 | Missing or unknown key |
| 403 | Key does not have the `write` scope |
| 404 | Container was not found |
| 429 | More than 60 requests in the last minute for this key |
