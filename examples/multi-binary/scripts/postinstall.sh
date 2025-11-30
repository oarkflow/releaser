#!/bin/bash
# Post-installation script for MyApp

set -e

# Create config directory if it doesn't exist
mkdir -p /etc/myapp

# Create log directory
mkdir -p /var/log/myapp

# Set up desktop integration for GUI app
if [ -d /usr/share/applications ]; then
    # Update desktop database
    if command -v update-desktop-database &> /dev/null; then
        update-desktop-database /usr/share/applications 2>/dev/null || true
    fi
fi

# Update icon cache
if [ -d /usr/share/icons/hicolor ]; then
    if command -v gtk-update-icon-cache &> /dev/null; then
        gtk-update-icon-cache -f /usr/share/icons/hicolor 2>/dev/null || true
    fi
fi

echo "MyApp installed successfully!"
echo "  CLI: Run 'myapp --help' for usage"
echo "  GUI: Launch 'MyApp' from your application menu"

exit 0
