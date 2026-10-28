# Notifications

Configure under **Settings → Update notifications**. Webhooks fire when Dockyard finds **pending image updates** (not on every scan if nothing changed — instant mode uses a fingerprint of pending rows).

**Types:** Discord, Slack, Microsoft Teams (Workflows Adaptive Card), or **Custom** JSON with template fields.

**Schedule:**

- **Instant** — when the pending set changes.
- **Interval** — repeat while updates remain (3h, 6h, 12h, 24h).
- **Daily** — once per day at a set local time.

Set an optional **Public URL** so messages can link to `{public_url}/updates`.

Use **Test** on a webhook to send a sample payload. Templates support `{{count}}`, `{{summary}}`, `{{names}}`, `{{link}}`, and more — see the Settings hint text.
