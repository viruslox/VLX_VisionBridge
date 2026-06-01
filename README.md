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

## Configuration Concepts

- **Canvas Size vs. Output Size**: The fundamental drawing area for layers and overlays is controlled by `input.resolution` (`InputSettings`). The final resolution of the stream that is encoded and pushed to your destinations is controlled by `output.resolution` (`OutputSettings`).

## Technology Stack

- **Language**: Go (Golang)
- **Processing Engine**: FFmpeg (via os/exec)
- **Database**: SQLite (State persistence, Logs, Metadata)
- **Messaging**: ZMQ is a mandatory dependency for real-time filter communication with FFmpeg.

See [Architecture](ARCHITECTURE.md) for High-Level Design details.
