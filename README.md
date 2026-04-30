# Project Design: Go-Live Orchestrator

## Project Overview

Go-Live Orchestrator is a headless, high-performance Linux service written in Go. It aggregates multiple finite SRT/WebRTC/Media streams into a single composite live stream, broadcasted simultaneously to multiple CDNs (YouTube, Twitch, VK).

The service is designed for professional 24/7 broadcasting environments where configuration must be dynamic and resource efficiency is paramount. We are basically building a sort of obs-studio for remote VMs.

## Requirements Note

- **Hardware**: Multi-core CPU for FFmpeg processing, adequate RAM for media buffering.
- **Software**: Modern Linux distribution (e.g., Ubuntu 20.04/22.04), FFmpeg installed and accessible.
- **Network**: High-bandwidth, low-latency network connection to handle multiple SRT/WebRTC streams and simultaneous broadcasting to multiple CDNs.

## Core Principles

- **Headless First**: Managed entirely via configuration files or DB entries.
- **Dynamic Reconfiguration**: Hot-reloading of layouts and sources without dropping the output stream (where technically possible).
- **Resource Optimization**: Sources marked as "OFF" are completely excluded from the processing pipeline.
- **Multi-Destination**: Single encoding pass with multiple output clones.

## Architecture Components

1. **Config Watcher**: Monitors `config.yaml` using `fsnotify`.
2. **State Manager**: Orchestrates the current state between the Config File and PostgreSQL.
3. **FFmpeg Engine**: A Go wrapper that generates and manages a complex subprocess for video/audio composition.
4. **Health Monitor**: Monitors CPU/RAM usage and stream stability, logging metrics to PostgreSQL.

## Technology Stack

- **Language**: Go (Golang)
- **Processing Engine**: FFmpeg (via os/exec or CGO bindings)
- **Database**: PostgreSQL (State persistence, Logs, Metadata)
- **Messaging (Optional)**: ZMQ for real-time filter communication with FFmpeg.

# High-Level Design (HLD)

*Note: This architecture is essentially building a headless OBS Studio tailored for remote VMs.*

## 1. System Workflow

The service initializes by reading the configuration. It builds a "Filter Graph" representing the 10 layers.

- **Input Layer**: Handles SRT handshake, file looping, or image buffer.
- **Processing Layer**: Scales, positions, and overlays inputs based on X/Y coordinates and percentage-based sizing.
- **Output Layer**: Encodes the composite frames into H.264/AAC and pushes to the `tee` muxer.

## 2. Dynamic Layer Management

Each of the 10 layers is treated as an independent object in the Go logic, mapping conceptually to OBS Studio's "Sources" within a "Scene" (Layout):

- **State**: `Active` | `Inactive`
- **Media**: `Video+Audio` | `Video Only` | `Audio Only`
- **Transform**: `Scale`, `Crop`, `Position` (with 5% default margin logic).

## 3. Database Schema (PostgreSQL)

While the `config.yaml` is the primary trigger, PostgreSQL stores:

- **Source Templates**: Reusable SRT URLs or file paths.
- **Layout Presets**: Saved configurations of the 10 layers.
- **Broadcast Logs**: Uptime, bitrate fluctuations, and error states.

## 4. Concurrency Model

Go routines are used to:

1. Manage the FFmpeg process lifecycle.
2. Listen for OS signals (SIGTERM, SIGHUP).
3. Update the DB with real-time health stats.
4. Watch for file system changes.

## 5. Output Specifications

Configurable parameters per session:

- **Resolution**: (e.g., 1920x1080)
- **FPS**: (e.g., 30, 60)
- **Video Bitrate**: (e.g., 6000k)
- **Audio Bitrate**: (e.g., 160k)
