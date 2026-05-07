#!/bin/bash
# Install opensearch-file-api as a systemd service.
# Run as root: sudo ./install.sh
#
# Optional: pass BINARY_PATH to override the default built binary location.
#   sudo BINARY_PATH=./bin/opensearch-file-api ./install.sh

set -euo pipefail

BINARY_NAME="opensearch-file-api"
BINARY_PATH="${BINARY_PATH:-./bin/${BINARY_NAME}}"
INSTALL_DIR="/usr/local/bin"
CONF_DIR="/etc/${BINARY_NAME}"
DATA_DIR="/var/lib/${BINARY_NAME}"
LOG_DIR="/var/log/${BINARY_NAME}"
SERVICE_FILE="/etc/systemd/system/${BINARY_NAME}.service"
SYSTEMD_UNIT="$(dirname "$0")/${BINARY_NAME}.service"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

if [[ $EUID -ne 0 ]]; then
    error "This script must be run as root (use sudo)"
    exit 1
fi

# 1. Build binary if not present
if [[ ! -f "$BINARY_PATH" ]]; then
    info "Binary not found at ${BINARY_PATH}, building..."
    make build
    if [[ ! -f "$BINARY_PATH" ]]; then
        error "Build failed: ${BINARY_PATH} still not found"
        exit 1
    fi
fi

# 2. Install binary
info "Installing binary to ${INSTALL_DIR}/${BINARY_NAME}"
cp "$BINARY_PATH" "${INSTALL_DIR}/${BINARY_NAME}"
chmod 755 "${INSTALL_DIR}/${BINARY_NAME}"

# 3. Create system user
if ! id "${BINARY_NAME}" &>/dev/null; then
    info "Creating system user '${BINARY_NAME}'"
    useradd --system --no-create-home --shell /usr/sbin/nologin "${BINARY_NAME}"
else
    info "System user '${BINARY_NAME}' already exists"
fi

# 4. Create directories
for dir in "$CONF_DIR" "$DATA_DIR" "$LOG_DIR"; do
    if [[ ! -d "$dir" ]]; then
        info "Creating directory ${dir}"
        mkdir -p "$dir"
    fi
    chown "${BINARY_NAME}:${BINARY_NAME}" "$dir"
done

# 5. Install config (if not already present)
if [[ ! -f "${CONF_DIR}/config.yaml" ]]; then
    info "Installing example config to ${CONF_DIR}/config.yaml"
    cp -f "$(dirname "$0")/../../config.example.yaml" "${CONF_DIR}/config.yaml"
    chown "${BINARY_NAME}:${BINARY_NAME}" "${CONF_DIR}/config.yaml"
    chmod 640 "${CONF_DIR}/config.yaml"
else
    info "Config already exists at ${CONF_DIR}/config.yaml, skipping"
fi

# 6. Create env file template (if not present)
if [[ ! -f "${CONF_DIR}/env" ]]; then
    info "Creating empty env file at ${CONF_DIR}/env"
    touch "${CONF_DIR}/env"
    chown "${BINARY_NAME}:${BINARY_NAME}" "${CONF_DIR}/env"
    chmod 640 "${CONF_DIR}/env"
fi

# 7. Install systemd service
info "Installing systemd service unit"
cp "$SYSTEMD_UNIT" "$SERVICE_FILE"
chmod 644 "$SERVICE_FILE"

# 8. Reload and enable
info "Reloading systemd daemon"
systemctl daemon-reload
info "Enabling ${BINARY_NAME} service"
systemctl enable "${BINARY_NAME}.service"

info "Installation complete!"
info ""
info "Next steps:"
info "  1. Edit config:  vim ${CONF_DIR}/config.yaml"
info "  2. (Optional) Set env vars: vim ${CONF_DIR}/env"
info "  3. Start service: systemctl start ${BINARY_NAME}"
info "  4. View logs:     journalctl -u ${BINARY_NAME} -f"
