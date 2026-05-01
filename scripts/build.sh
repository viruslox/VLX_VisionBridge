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

if ! command -v ffmpeg &> /dev/null; then
    echo "Error: ffmpeg is not installed or not in PATH."
    exit 1
fi

echo "Building executable..."
go build -o ./go-live-orchestrator cmd/server/main.go
echo "Build successful."

echo "Generating config template..."
mkdir -p configs
cat << 'CONFIG_EOF' > configs/config.yaml.template
# Configuration for Go-Live Orchestrator

# Global output settings for the stream
output:
  resolution: "1920x1080" # Target output resolution
  fps: 30                 # Target output framerate
  video_bitrate: "6000k"  # Output video bitrate
  audio_bitrate: "160k"   # Output audio bitrate
  destinations:           # List of RTMP endpoints to stream to
    - "rtmp://localhost/live/test"

# Layers (0-9) configured as inputs and their layouts
layers:
  - id: 0                    # Layer ID
    active: true             # Whether this layer is processed
    input_type: "loop"       # Type: 'loop', 'folder', 'srt'
    input_path: "./test.mp4" # Path to the file or SRT address
    media: "Video+Audio"     # Which tracks to include: Video+Audio, Video Only, Audio Only
    scale: "1920:-1"         # Scaling options
    crop: "1920:1080:0:0"    # Cropping options
    position: "center"       # Position on canvas: center, top-left
CONFIG_EOF

echo "Config template generated at configs/config.yaml.template."
