#!/bin/bash
# Post-installation script for GoFiber API

set -e

# Create system user if it doesn't exist
if ! getent group gofiber-api > /dev/null 2>&1; then
    groupadd --system gofiber-api
fi

if ! getent passwd gofiber-api > /dev/null 2>&1; then
    useradd --system --gid gofiber-api --no-create-home --shell /usr/sbin/nologin gofiber-api
fi

# Create directories
mkdir -p /etc/gofiber-api
mkdir -p /var/log/gofiber-api

# Set permissions
chown -R gofiber-api:gofiber-api /etc/gofiber-api
chown -R gofiber-api:gofiber-api /var/log/gofiber-api
chmod 750 /etc/gofiber-api
chmod 750 /var/log/gofiber-api

# Copy default config if not exists
if [ ! -f /etc/gofiber-api/config.yaml ]; then
    if [ -f /usr/share/gofiber-api/config.example.yaml ]; then
        cp /usr/share/gofiber-api/config.example.yaml /etc/gofiber-api/config.yaml
        chown gofiber-api:gofiber-api /etc/gofiber-api/config.yaml
        chmod 640 /etc/gofiber-api/config.yaml
    fi
fi

# Install systemd service
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload
    systemctl enable gofiber-api.service || true
    echo "GoFiber API installed. Start with: systemctl start gofiber-api"
fi

echo "GoFiber API installation complete!"
