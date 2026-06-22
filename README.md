# Project Design: VLX VisionBridge

## Project Overview

VLX VisionBridge is a headless, high-performance Linux service written in Go. It aggregates multiple finite SRT/WebRTC/Media streams into a single composite live stream, broadcasted simultaneously to multiple CDNs (YouTube, Twitch, VK).

The service is designed for professional 24/7 broadcasting environments where configuration must be dynamic and resource efficiency is paramount. We are basically building a sort of obs-studio for remote VMs.

## How it Works

VisionBridge employs a highly optimized, Cloud-Native "Sidecar" Architecture based on three pillars:
1. **WebRTC Overlay**: Chromium runs Headless, captures its own canvas (preserving Alpha channel), and pushes VP8/Opus via WebRTC (Pion) to local UDP ports.
2. **GStreamer Core**: A native GStreamer pipeline mixes WebRTC and external media with zero latency.
3. **Local Proxy (Sidecar)**: GStreamer muxes the output and pushes it unencrypted to a local MediaMTX server (`rtmp://127.0.0.1:1935/live/internal`). External routing and TLS are handled dynamically by Chatbridge invoking MediaMTX REST APIs.

## Requirements Note

- **Hardware**: Multi-core CPU for GStreamer processing, adequate RAM for media buffering.
- **Software**: Modern Linux distribution (e.g., Ubuntu 20.04/22.04), GStreamer 1.0 (with good/bad/ugly plugins and libav) installed and accessible, Chromium (optional, if using overlay HTML sources), and `pion/webrtc`.
- **Network**: High-bandwidth, low-latency network connection to handle multiple SRT/WebRTC streams and simultaneous broadcasting.

```bash
apt-get install gstreamer1.0-tools gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad gstreamer1.0-plugins-ugly gstreamer1.0-libav
```

## Core Principles

- **Zero-Latency Hybrid Pipeline**: Utilizes a highly optimized WebRTC/GStreamer Hybrid architecture.
- **Headless First**: Managed entirely via configuration files or DB entries.
- **Dynamic Reconfiguration**: Hot-reloading of layouts and sources without dropping the output stream (where technically possible).
- **Resource Optimization**: Sources marked as "OFF" are completely excluded from the processing pipeline.
- **Multi-Destination**: Single encoding pass with multiple output clones.

## Best Practices

- **Top-Most Overlay**: The `chromium_source` overlay should be kept as the top-most layer in the filter chain to avoid performance issues associated with complex layering. It is automatically rendered on top of all other sources.
- **Layer Conventions**: To ensure a stable and performance-oriented configuration, `media_source` is limited to a maximum of 3 layers (IDs 0, 1, 2) which share identical capabilities. The recommended convention is:
  - **Layer 1**: Primary Input (e.g., GoPro).
  - **Layer 0**: Fallback/Placeholder (e.g., VODs).
  - **Layer 2**: Secondary Input / Guest.

## Layer Configuration Examples

VisionBridge operates alongside MediaMTX and ChatBridge on the same localhost. It handles low-latency video ingestion and real-time scene switching via ZeroMQ without restarting GStreamer.

### Folder Playlist Input
When the input path is a directory and the layer configuration includes `folder_options` with `is_folder: true`, VisionBridge treats it as a playlist. It plays all valid video files (e.g., MP4, WebM) found in the directory. You can shuffle the playlist, loop it, and insert a delay (customizable delay spacer (color or image-based)) between videos.
```yaml
id: 0
active: true
input_type: "local"
input_path: "/opt/VLX_VisionBridge/data/layer0"
media: "Video+Audio"
size: 1920
x: 0
y: 0
volume: 100
folder_options:
  is_folder: true
  shuffle: true
  loop: true
  delay_sec: 5
  spacer_width: 1920
  spacer_height: 1080
  spacer_fps: 30
  spacer_sample_rate: 48000
  spacer_color: "black"
  spacer_image: ""
```

### Network Input
Optimized for MediaMTX via RTSP/SRT. The `network` input_type is suitable for true zero-latency ingestion from MediaMTX (SRT/RTSP).
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

## Layer Control Rules ##

- **Rule 1:** ZMQ commands MUST ONLY target `media_source` layers (hardware cameras, local videos).
- **Rule 2:** `chromium_source` layers (Overlays, Alerts, Maps) MUST be kept `active: true` constantly. Show/Hide logic for web layers must be handled via WebSockets/JavaScript, NOT via ZMQ, to avoid GStreamer restarts and stream drops.

## Dynamic Regia

Scenes are toggled on/off screen by moving them to `x -9999` and setting `volume 0.0` via ZMQ. This mechanism guarantees zero frame drops when switching scenes since GStreamer doesn't have to restart or reload inputs.

## Configuration Concepts

- **Canvas Size vs. Output Size**: The fundamental drawing area for layers and overlays is controlled by `input.resolution` (`InputSettings`). The final resolution of the stream that is encoded and pushed to your destinations is controlled by `output.resolution` (`OutputSettings`).

## Technology Stack

- **Language**: Go (Golang)
- **Processing Engine**: GStreamer (via os/exec)
- **Database**: SQLite (State persistence, Logs, Metadata)
- **Messaging**: ZMQ is a mandatory dependency for real-time filter communication with GStreamer.

See [Architecture](ARCHITECTURE.md) for High-Level Design details.
