#!/bin/bash
set -e

if [ "$EUID" -eq 0 ]; then
    echo "Error: Please do not run this install script as root."
    exit 1
fi

if [ ! -f "./VLX_VisionBridge" ]; then
    echo "Error: Executable not found. Please run scripts/build.sh first."
    exit 1
fi

if [ ! -f "./configs/config.yaml.template" ]; then
    echo "Error: Configuration template not found. Please run scripts/build.sh first."
    exit 1
fi

# Prompt for installation path
DEFAULT_PATH="$HOME/VLX_VisionBridge"
read -p "Enter installation path [$DEFAULT_PATH]: " INSTALL_PATH
INSTALL_PATH=${INSTALL_PATH:-$DEFAULT_PATH}

echo "Installing to $INSTALL_PATH..."

mkdir -p "$INSTALL_PATH/configs"
mkdir -p "$INSTALL_PATH/bin"

cp ./VLX_VisionBridge "$INSTALL_PATH/bin/"
cp ./configs/config.yaml.template "$INSTALL_PATH/configs/config.yaml"

echo "Installation complete."
echo "Executable is at: $INSTALL_PATH/bin/VLX_VisionBridge"
echo "Configuration is at: $INSTALL_PATH/configs/config.yaml"
