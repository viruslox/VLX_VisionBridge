# Project Design: VLX VisionBridge

## Project Overview

VLX VisionBridge is a headless, high-performance Linux service written in Go. VLX_VisionBridge utilizes a DOM-dominant Architecture: All media rendering (videos, images, carousels) happens exclusively in the Chromium DOM. GStreamer acts solely as a passive X11/PulseAudio screen recorder pushing to MediaMTX, while WebRTC captures internal chromium_source signaling.

The service is designed for professional 24/7 broadcasting environments where configuration must be dynamic and resource efficiency is paramount. We are basically building a sort of obs-studio for remote VMs.

## How it Works

VisionBridge employs a highly optimized, Cloud-Native "Sidecar" Architecture based on three pillars:
1. **DOM-dominant Architecture**: All media rendering (videos, images, carousels) happens exclusively in the Chromium DOM.
2. **GStreamer Core**: GStreamer acts solely as a passive X11/PulseAudio screen recorder pushing to MediaMTX, while WebRTC captures internal `chromium_source` signaling.
3. **Local Proxy (Sidecar)**: GStreamer muxes the output and pushes it unencrypted to a local MediaMTX server (`rtmp://127.0.0.1:1935/live/internal`). External routing and TLS are handled dynamically by Chatbridge invoking MediaMTX REST APIs.

## Requirements Note

- **Hardware**: Multi-core CPU for GStreamer processing, adequate RAM for media buffering.
- **Software**: Modern Linux distribution (e.g., Ubuntu 20.04/22.04), GStreamer 1.0 (with good/bad/ugly plugins and libav) installed and accessible, Chromium (optional, if using overlay HTML sources), and `pion/webrtc`.
- **Network**: High-bandwidth, low-latency network connection to handle multiple SRT/WebRTC streams and simultaneous broadcasting.

```bash
apt-get install gstreamer1.0-tools gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad gstreamer1.0-plugins-ugly gstreamer1.0-libav
```

## Core Principles

- **DOM-dominant Architecture**: All media rendering happens exclusively in the Chromium DOM. GStreamer acts solely as a passive X11/PulseAudio screen recorder.
- **Headless First**: Managed entirely via configuration files or DB entries.
- **Dynamic Reconfiguration**: Hot-reloading of layouts and sources without dropping the output stream (where technically possible).
- **Resource Optimization**: Sources marked as "OFF" are completely excluded from the processing pipeline.
- **Multi-Destination**: Single encoding pass with multiple output clones.

## Best Practices

VisionBridge operates alongside MediaMTX and ChatBridge on the same localhost.

The `input` configuration revolves around `chromium_source`, which supports exactly 8 distinct Z-layers (`Z1` to `Z8`) with explicit `height`, `width`, `x`, `y`, `volume`, and `path` variables. It dynamically generates HTML tags (`<video autoplay loop>`, `<img>`) based on the content type inferred from the path.

For directory-based media playback in `chromium_source`, the Go backend provides an HTTP endpoint (`/api/list-dir?path=...`) that the Chromium WebSocket client fetches to automatically sequence and loop media as a carousel without GStreamer intervention.

```yaml
input:
  resolution: "1920x1080"
  chromium_source:
    active: true
    z1_active: true
    z1_path: "/opt/VLX_VisionBridge/data/layer1.mp4"
    z1_volume: 100
    z1_width: 1920
    z1_height: 1080
    z1_x: 0
    z1_y: 0
```

## Layer Control Rules

- **Rule 1:** Incoming JSON control commands for overlays are parsed and broadcasted as WebSocket messages (e.g., `{"layer": "z1", "action": "play", "path": "..."}`) to Chromium clients for zero-CPU DOM manipulation.
- **Rule 2:** `chromium_source` layers MUST be kept `active: true` constantly. Show/Hide logic for web layers must be handled via WebSockets/JavaScript to avoid stream drops. Setting the target stream to disabled (`Enabled: false`) triggers a process termination to halt the active stream.

## Configuration Concepts

- **Canvas Size vs. Output Size**: The fundamental drawing area for layers and overlays is controlled by `input.resolution` (`InputSettings`). The final resolution of the stream that is encoded and pushed to your destinations is controlled by `output.resolution` (`OutputSettings`).

## Technology Stack

- **Language**: Go (Golang)
- **Processing Engine**: GStreamer (via os/exec)
- **Database**: SQLite (State persistence, Logs, Metadata)
- **Messaging**: ZeroMQ JSON control commands are parsed and broadcasted as WebSocket messages to Chromium clients.

See [Architecture](ARCHITECTURE.md) for High-Level Design details.
