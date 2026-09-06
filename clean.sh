#!/bin/bash
set -e

echo "Cleaning VisionBridge build artifacts..."
rm -f VLX_VisionBridge VLX_VisionBridge_frontend
rm -rf frontend_app/node_modules
rm -rf frontend_app/dist
rm -rf internal/ui/dist
echo "Clean complete."
