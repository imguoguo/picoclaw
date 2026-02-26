#!/bin/sh
set -e

PICOCLAW_DIR="/home/picoclaw/.picoclaw"

# Running as root means the bind-mounted data directory was created by Docker
# (owned by root). Fix ownership then drop to the picoclaw user and re-exec.
if [ "$(id -u)" = "0" ]; then
    mkdir -p "$PICOCLAW_DIR"
    chown -R picoclaw:picoclaw "$PICOCLAW_DIR"
    exec su-exec picoclaw env HOME=/home/picoclaw "$0" "$@"
fi

# First-run: neither config nor workspace exists.
# If config.json is already mounted but workspace is missing we skip onboard to
# avoid the interactive "Overwrite? (y/n)" prompt hanging in a non-TTY container.
if [ ! -d "${HOME}/.picoclaw/workspace" ] && [ ! -f "${HOME}/.picoclaw/config.json" ]; then
    picoclaw onboard
    echo ""
    echo "First-run setup complete."
    echo "Edit ${HOME}/.picoclaw/config.json (add your API key, etc.) then restart the container."
    exit 0
fi

exec picoclaw gateway "$@"
