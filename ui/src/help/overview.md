# Overview

Dockyard is a local-first Docker manager for Linux: one static binary, a **k9s-style TUI** by default, and an optional **Vue web UI** (`--web` or `--headless --web`).

It talks to the Docker socket (usually `/var/run/docker.sock`), keeps container state fresh with **Docker events**, polls **live stats** for sparklines and graphs, and flags **restart loops** when a container keeps restarting.

Run it interactively on a laptop, or under **systemd** headless with the web UI on loopback. The same engine powers the TUI and the web UI; do not run two full engines against the same host unless you know why - each process has its own auto-update loop.
