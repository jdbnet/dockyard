#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${CONFIG_DIR:-/etc/dockyard}"
SERVICE_NAME="dockyard"

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  echo "Run as root or with sudo"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ -f "$ROOT_DIR/dockyard" ]]; then
  BINARY="$ROOT_DIR/dockyard"
elif command -v dockyard >/dev/null 2>&1; then
  BINARY="$(command -v dockyard)"
else
  echo "Build first: go build -o dockyard ./cmd/dockyard"
  exit 1
fi

install -m 755 "$BINARY" "$INSTALL_DIR/dockyard"
mkdir -p "$CONFIG_DIR"

if [[ ! -f "$CONFIG_DIR/config.yaml" ]]; then
  install -m 644 "$ROOT_DIR/config.yaml" "$CONFIG_DIR/config.yaml"
  echo "Installed default config to $CONFIG_DIR/config.yaml"
fi

if ! id dockyard &>/dev/null; then
  useradd --system --no-create-home --shell /usr/sbin/nologin dockyard || true
fi
usermod -aG docker dockyard 2>/dev/null || true

install -m 644 "$SCRIPT_DIR/dockyard.service" "/etc/systemd/system/${SERVICE_NAME}.service"
systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"

echo "Dockyard installed. Web UI: http://127.0.0.1:8080 (edit $CONFIG_DIR/config.yaml)"
echo "Status: systemctl status $SERVICE_NAME"
