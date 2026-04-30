# Project Design: Go-Live Orchestrator

## Project Overview
Go-Live Orchestrator is a headless, high-performance Linux service written in Go. It aggregates multiple finite SRT/WebRTC/Media streams into a single composite live stream, broadcasted simultaneously to multiple CDNs (YouTube, Twitch, VK). 

The service is designed for professional 24/7 broadcasting environments where configuration must be dynamic and resource efficiency is paramount.

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