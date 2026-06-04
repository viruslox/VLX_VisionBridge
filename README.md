# Project Design: VLX VisionBridge

## Project Overview

VLX VisionBridge is a headless, high-performance Linux service written in Go. It aggregates multiple finite SRT/WebRTC/Media streams into a single composite live stream, broadcasted simultaneously to multiple CDNs (YouTube, Twitch, VK).

The service is designed for professional 24/7 broadcasting environments where configuration must be dynamic and resource efficiency is paramount. We are basically building a sort of obs-studio for remote VMs.

## Requirements Note

- **Hardware**: Multi-core CPU for FFmpeg processing, adequate RAM for media buffering.
- **Software**: Modern Linux distribution (e.g., Ubuntu 20.04/22.04), FFmpeg installed and accessible. Xvfb and Chromium (optional, if using overlay HTML sources).
- **Network**: High-bandwidth, low-latency network connection to handle multiple SRT/WebRTC streams and simultaneous broadcasting to multiple CDNs.

## Core Principles

- **Headless First**: Managed entirely via configuration files or DB entries.
- **Dynamic Reconfiguration**: Hot-reloading of layouts and sources without dropping the output stream (where technically possible).
- **Resource Optimization**: Sources marked as "OFF" are completely excluded from the processing pipeline.
- **Multi-Destination**: Single encoding pass with multiple output clones.

## Layer Configuration Examples

VisionBridge operates alongside MediaMTX and ChatBridge on the same localhost. It handles low-latency video ingestion and real-time scene switching via ZeroMQ without restarting FFmpeg.

### IPC Audio Input
Accepts raw PCM data directly via a Unix Domain Socket. If `InputPath` is left empty, it defaults to `/tmp/vlx_audio.sock`, but it can also be explicitly defined.
```json
{
  "id": 1,
  "active": true,
  "input_type": "ipc_audio",
  "input_path": "",
  "media": "Audio",
  "volume": 1.0
}
```

### Network Input
Optimized for MediaMTX via RTSP/SRT. The `network` input_type automatically injects `-fflags nobuffer -flags low_delay` for true zero-latency ingestion from MediaMTX (SRT/RTSP).
```json
{
  "id": 2,
  "active": true,
  "input_type": "network",
  "input_path": "rtsp://localhost:8554/stream",
  "media": "Video+Audio",
  "size": 1920,
  "x": 0,
  "y": 0,
  "volume": 1.0
}
```

## ⚠️ The Golden Rule of Layer Control ⚠️

- **Rule 1:** ZMQ commands MUST ONLY target `ffmpeg_source` layers (hardware cameras, local videos).
- **Rule 2:** `chromium_source` layers (Overlays, Alerts, Maps) MUST be kept `active: true` constantly. Show/Hide logic for web layers must be handled via WebSockets/JavaScript, NOT via ZMQ, to avoid FFmpeg restarts and stream drops.

## Dynamic Regia

Scenes are toggled on/off screen by moving them to `x -9999` and setting `volume 0.0` via ZMQ. This mechanism guarantees zero frame drops when switching scenes since FFmpeg doesn't have to restart or reload inputs.

## Configuration Concepts

- **Canvas Size vs. Output Size**: The fundamental drawing area for layers and overlays is controlled by `input.resolution` (`InputSettings`). The final resolution of the stream that is encoded and pushed to your destinations is controlled by `output.resolution` (`OutputSettings`).

## Technology Stack

- **Language**: Go (Golang)
- **Processing Engine**: FFmpeg (via os/exec)
- **Database**: SQLite (State persistence, Logs, Metadata)
- **Messaging**: ZMQ is a mandatory dependency for real-time filter communication with FFmpeg.

See [Architecture](ARCHITECTURE.md) for High-Level Design details.
