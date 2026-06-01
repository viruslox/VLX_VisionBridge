# Architecture: VLX VisionBridge

This document outlines the high-level design and architectural details of VLX VisionBridge.

## System Overview

VLX VisionBridge is a headless, high-performance Linux service written in Go. It essentially functions as a remote, headless OBS Studio tailored for remote VMs. It aggregates multiple finite SRT/WebRTC/Media streams into a single composite live stream, which is broadcasted simultaneously to multiple CDNs (YouTube, Twitch, VK).

## Project Structure

The project is structured according to common Go conventions, primarily using the `internal/` directory to encapsulate private logic:

- `internal/models`: Defines core structs such as `Layer`, `Config`, and `DatabaseConfig`.
- `internal/config`: Handles parsing of the `visionbridge.settings` YAML file and implements configuration watching and diffing.
- `internal/db`: Manages the SQLite database connection pool and logging queries.
- `internal/engine`: The core FFmpeg command generator and process manager. It is further decoupled into:
  - `source`: Prepares input arguments, path sanitization, and input file parsing.
  - `mixer`: Constructs complex filtergraphs and manages dynamic ZMQ updates.
  - `streamer`: Builds multi-destination output pipelines using the `tee` muxer.

## Configuration & Hot-Reloading

The service is fully configured via the `visionbridge.settings` file (default location: `/opt/VLX_VisionBridge/etc/visionbridge.settings`, but can be overridden by `CONFIG_PATH` or positional arguments).

The configuration is hot-reloadable. A `Config Watcher` uses `fsnotify` to monitor the settings file. When changes are detected, a diffing logic determines the required action:
- **RequiresFilterUpdate**: Changes to layout properties like `X`, `Y`, and `Volume` trigger a live filter update via ZMQ without dropping the stream.
- **RequiresRestart**: Changes to structural properties like `Size`, `Active`, or `Destinations` require a full FFmpeg process restart.

## Input Pipelines

The mixer coordinates two distinct input pipelines conceptually similar to "Sources" within a "Scene":

### 1. Standard Media Layers (`ffmpeg_source`)

Up to 10 independent objects managed directly via FFmpeg inputs.
- **State**: `Active` | `Inactive`
- **Input Type**: `local` (folder of media), `srt`, `rtmp` (and `rtmps`), `webrtc`, `rtsp` (and `rtsps`), `ipc_audio` (raw PCM over UDS). For `local`, folder combinations are automatically parsed (video only, image + audio, image only, audio only).
- **Media**: `Video+Audio` | `Video` | `Audio`
- **Transform**: `Size` (scale width), `X`, `Y` (Position).
- **Audio**: Configurable `Volume` per layer.

### 2. HTML Overlays (`chromium_source`)

An independently spawned Chromium process dynamically rendering up to 7 Z-layers (`Z1` to `Z7`), captured via `x11grab` and `xvfb-run` by FFmpeg.
- Handles standard web URLs, as well as automatic HTML `<video>` / `<img>` / `<audio>` tag generation for local media.
- Background colors can be dynamically injected into the generated HTML.
- Ensures absolute layout positioning via inline CSS directly to eliminate browser offsets.

## Engine & Mixer

The FFmpeg Mixer uses advanced `filter_complex` graphs to scale, position, and overlay inputs based on absolute integer-based pixel sizing and X/Y coordinates.

### VLX Connector (IPC Integration)
To eliminate local SRT network overhead and reduce latency for deployments running alongside `VLX_ChatBridge`, VisionBridge integrates a dedicated IPC connector:
- **Audio Ingress (`ipc_audio`)**: Accepts raw PCM data (`s16le`, 48kHz, 2-channel) directly via a Unix Domain Socket (`/tmp/vlx_audio.sock`), injecting it seamlessly into the FFmpeg audio mixer.
- **Control Ingress**: A listener on `/tmp/vlx_control.sock` accepts JSON command payloads, directly mutating the internal State Manager to toggle inputs and trigger actions without requiring Web browser overhead.

- **Dynamic Updates via ZMQ**: Live properties (like `overlay@layerID` coordinates and `volume@layerID`) are manipulated in real-time. The mixer binds a `zmq` filter to `tcp://127.0.0.1:5555` to receive string commands.
- **Performance Optimizations**: For performance-sensitive code paths in filter generation, `strings.Builder` and stack buffers are used over `fmt.Sprintf` to minimize memory allocations.

## Output Pipeline

The output layer encodes the composite frames into H.264/AAC and pushes to a robust multi-destination pipeline:
- **`tee` Muxer**: Used for simultaneous output cloning.
- **`fifo` Pseudo-muxer**: Integrated to isolate output streams. In the event a single destination (e.g., an unstable RTMP endpoint) fails, `fifo` combined with `:onfail=ignore` prevents the entire FFmpeg process from crashing.
- Automatic codec enforcement prevents conversion failures when mixing different media types or dummy streams.

## Resilience & Process Management

A robust `ProcessManager` governs the underlying FFmpeg subprocess:
- **Error Diagnostics**: Maintains a `tailBuffer` of the last 4096 bytes of the process's standard error stream to pinpoint failures (identifying them as `[input]`, `[mixer]`, or `[output]` issues).
- **RetryTracker**: Uses a backoff strategy (5 quick retries, 2 slow retries, then dynamic disablement) for isolating failures in sources like Chromium overlays.
- Process reaps and signal listeners ensure no zombie processes remain after graceful or ungraceful shutdown.

## Database

While `visionbridge.settings` triggers state changes, SQLite is the sole database backend used for persistence and telemetry.
- **Location**: Default path `/opt/VLX_VisionBridge/var/visionbridge.db`.
- **Usage**: Stores reusable source templates, layout presets, and broadcast logs (uptime, bitrate fluctuations, error states).

## Security

- Non-root execution constraints are explicitly enforced (except during initial installation).
- `SanitizeInputPath` prevents FFmpeg argument injection (e.g., ensuring paths starting with `-` are prefixed with `./`).
- Strict strict integer typing for overlay properties (Size, X, Y) prevents filter injection vulnerabilities.
