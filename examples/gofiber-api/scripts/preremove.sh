#!/bin/bash
# Pre-removal script for GoFiber API

set -e

# Stop the service if running
if [ -d /run/systemd/system ]; then
    systemctl stop gofiber-api.service || true
    systemctl disable gofiber-api.service || true
fi

echo "GoFiber API service stopped and disabled."
