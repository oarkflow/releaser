#!/bin/bash
# Pre-removal script for MyApp

set -e

echo "Removing MyApp..."

# Stop any running instances
if command -v pkill &> /dev/null; then
    pkill -9 myapp || true
    pkill -9 myapp-gui || true
fi

exit 0
