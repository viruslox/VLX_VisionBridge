#!/bin/bash
set -e

if [ "$EUID" -eq 0 ]; then
    echo "Error: Please do not run this build script as root."
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "Error: go is not installed or not in PATH."
    exit 1
fi

echo "Building executable..."
go build -o ./VLX_VisionBridge cmd/server/*.go
echo "Build successful."
echo "You can now run 'sudo ./VLX_VisionBridge install' to install."
