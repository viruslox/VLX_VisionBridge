#!/bin/bash
set -e

echo "Cleaning VisionBridge build artifacts and caches..."

# Remove main build outputs
rm -f VLX_VisionBridge VLX_VisionBridge_frontend server

# Remove frontend artifacts
rm -rf frontend_app/node_modules/
rm -rf frontend_app/dist/
rm -rf frontend_app/.svelte-kit/
rm -rf node_modules/
rm -rf internal/ui/dist/

# Clean common IDE / temporary files
find . -name "*.test" -type f -delete
find . -name "*.out" -type f -delete
find . -name "*.swp" -type f -delete
find . -name "*~" -type f -delete
find . -name ".DS_Store" -type f -delete

echo "Clean complete."
