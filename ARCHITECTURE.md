# Architecture: VLX VisionBridge

This document outlines the high-level design and architectural details of VLX VisionBridge.

## System Overview

VLX VisionBridge is a headless, high-performance Linux service written in Go. It essentially functions as a remote, headless OBS Studio tailored for remote VMs. VLX_VisionBridge utilizes a DOM-dominant Architecture: All media rendering (videos, images, carousels) happens exclusively in the Chromium DOM. GStreamer acts solely as a passive screen recorder using a static pipeline with `ximagesrc` (capturing Xvfb display :99) and `pulsesrc` pushing to MediaMTX.

### MediaMTX Sidecar Proxy Pattern

VisionBridge acts strictly as a low-latency video mixer pushing to `localhost` (`rtmp://127.0.0.1:1935/live/internal`), delegating all RTMPS/TLS network resilience and dynamic external routing to the local MediaMTX instance (controlled via REST APIs by Chatbridge).

## Project Structure

The project is structured according to common Go conventions, primarily using the `internal/` directory to encapsulate private logic:

- `internal/models`: Defines core structs such as `Layer`, `Config`, and `DatabaseConfig`.
- `configs`: Contains the `configs/visionbridge.settings.template` embedded directly into the binary via `configs/assets.go` to facilitate self-contained installations.
- `internal/config`: Handles parsing of the `visionbridge.settings` YAML file and implements configuration watching and diffing.
- `internal/db`: Manages the SQLite database connection pool (using `github.com/mattn/go-sqlite3`) and logging queries.
- `internal/engine`: The core GStreamer pipeline generator and process manager. It is further decoupled into:
  - `source`: Prepares input arguments, path sanitization, and input file parsing.
  - `mixer`: Constructs complex pipelines and manages inputs.
  - `streamer`: Builds multi-destination output pipelines.

## Configuration & Hot-Reloading

The service is fully configured via the `visionbridge.settings` file (default location: `/opt/VLX_VisionBridge/etc/visionbridge.settings`, but can be overridden by `CONFIG_PATH` or positional arguments).

The configuration is hot-reloadable. A `Config Watcher` uses `fsnotify` to monitor the settings file. When changes are detected, a diffing logic determines the required action:

- **RequiresFilterUpdate**: Layout property changes (like X, Y, Volume) map to a Single Source of Truth (SSOT) pattern updating the YAML settings file directly, and the file watcher triggers WebSocket broadcasts to the Chromium DOM for zero-CPU DOM manipulation.
- **RequiresRestart**: Changes to structural properties like `Resolution`, `Active`, or `Destinations` require a full GStreamer process restart.

### Configuration Reference

VisionBridge is configured via a YAML settings file containing five primary sections:

#### Database

- `dsn`: The Data Source Name or path to the SQLite database file.

#### Connector

- `ipc_control_in`: Boolean to enable or disable IPC control socket.
- `group`: The user group assigned to the control socket for permission access.
- `control_socket`: The file path for the Unix domain control socket.

#### Output

- `active`: Boolean to toggle the streaming output state.
- `resolution`: The final scaled output resolution (e.g., "1920x1080").
- `fps`: Target frames per second.
- `video_bitrate`: Target video encoding bitrate.
- `audio_bitrate`: Target audio encoding bitrate.
- `audio_sample_rate`: Target audio sample rate.
- `destinations`: Array of destination URIs to push the stream to.

#### Input

- `bg_color`: The global background color (e.g., "black") used in the Chromium overlay.
- `resolution`: The base canvas resolution for rendering layers.
- `framerate`: The processing framerate for inputs.
- `carousel_delay`: Sleep duration between sequential media playbacks in milliseconds.
- `carousel_shuffle`: Boolean to randomize carousel playback order.
- `webrtc_port_min`: Minimum ephemeral UDP port for WebRTC signaling.
- `webrtc_port_max`: Maximum ephemeral UDP port for WebRTC signaling.
- `overlay_server_active`: Boolean to enable the internal Web/WebSocket server.
- `overlay_server_port`: Port for the internal Web/WebSocket server.
- `media_folder_path`: Base path for the media directory to serve static assets.
- `chromium_source`: A block that configures up to 13 native DOM Z-layers (`Z0` to `Z12`).
  - `active`: Boolean to keep the entire Chromium layer active.
  - `z*_active`: Boolean to enable/disable a specific layer (e.g., `z0_active`).
  - `z*_path`: File path, directory path, or URL for the media source.
  - `z*_volume`: Volume (0-100) for native media elements.
  - `z*_width`, `z*_height`: Width and height dimensions of the layer.
  - `z*_x`, `z*_y`: X and Y absolute layout coordinates of the layer.

#### Control API

- `enable`: Boolean to enable or disable the control API.
- `bind_address`: The IP address to bind the API to.
- `port`: The port to bind the API to.
- `user`: The Basic Auth username for the API.
- `pass`: The Basic Auth password for the API.
- `log_unit`: The systemd log unit used for fetching logs in the UI.

## Input Pipelines

The system coordinates a single DOM-dominant pipeline conceptually similar to a "Scene":

### HTML Overlays (`chromium_source`)

An independently spawned Chromium process running in non-headless mode inside an Xvfb display dynamically rendering up to 13 Z-layers (`Z0` to `Z12`).

- All media rendering (videos, images, carousels) happens exclusively in the Chromium DOM.
- It dynamically generates HTML tags (`<video autoplay loop>`, `<img>`, `<iframe>`) based on the content type inferred from the path.
- For directory-based media playback, the Go backend provides an HTTP endpoint (`/api/list-dir?path=...`) that the Chromium WebSocket client fetches to automatically sequence and loop media as a carousel without GStreamer intervention.
- Background colors can be dynamically injected into the generated HTML via `BgColor` under `InputSettings`.
- Ensures absolute layout positioning via inline CSS directly to eliminate browser offsets.

## Engine & Mixer

GStreamer acts solely as a passive screen recorder using a static pipeline with `ximagesrc` (capturing Xvfb display :99) and `pulsesrc` pushing to MediaMTX.

The base canvas size for the pipeline is defined by `cfg.Input.Resolution` within the `InputSettings`. This establishes the drawing area for all overlays and layers. The final scaled output, which is sent to external destinations, is defined separately by `cfg.Output.Resolution`. Because the input resolution dictates the fundamental structure of the pipeline and video buffers, any changes to the input resolution require a full restart, whereas changes to individual layer positions or sizes may not.

### VLX Connector (IPC Integration)

To eliminate local SRT network overhead and reduce latency for deployments running alongside `VLX_ChatBridge`, VisionBridge integrates a dedicated IPC connector:

- **Control Ingress**: A listener on the Unix control socket (`/tmp/vlx_control.sock`) handles incoming control messages. Incoming JSON control commands for overlays are processed using a Single Source of Truth (SSOT) pattern: JSON commands update the YAML settings file directly, and the file watcher triggers WebSocket broadcasts to the Chromium clients for zero-CPU DOM manipulation.
- **Auto-Fallback Concept**: Users can hook `runOnPublish` / `runOnUnpublish` scripts in MediaMTX to inject JSON into VisionBridge's control socket. This allows creating an automatic "Be Right Back" screen or fallback sequence upon signal loss.

- **Dynamic Updates via WebSocket**: Live properties are manipulated in real-time. Control commands sent to the configured control socket map to the `ControlCommand` struct, containing `event_id`, `timestamp`, `action` (e.g., 'set_input_state'), `target`, and a `payload` object.
- **Performance Optimizations**: For performance-sensitive code paths in filter generation, `strings.Builder` and stack buffers are used over `fmt.Sprintf` to minimize memory allocations.

## Output Pipeline

The output layer encodes the composite frames into H.264/AAC and pushes to a robust local destination pipeline:

- **`tee` Muxer**: Used for simultaneous output cloning if needed.
- Automatic codec enforcement prevents conversion failures when mixing different media types or dummy streams.

## Resilience & Process Management

A robust `ProcessManager` governs the underlying GStreamer subprocess:

- **Idle Behavior**: If the stream output is deactivated via configuration or IPC, the ProcessManager keeps GStreamer fully dormant (consuming 0% CPU) until a JSON `ControlCommand` with `target='stream'` and `enabled=true` wakes it up.
- **Health Monitor**: Monitors CPU/RAM usage and stream stability, logging metrics to SQLite.
- **Error Diagnostics**: Maintains a `tailBuffer` of the last 4096 bytes of the process's standard error stream to pinpoint failures (identifying them as `[input]`, `[mixer]`, or `[output]` issues).
- **RetryTracker**: Uses a backoff strategy (5 quick retries, 2 slow retries, then dynamic disablement) for isolating failures in sources like Chromium overlays.
- Process reaps and signal listeners ensure no zombie processes remain after graceful or ungraceful shutdown.

## Persistence and Logging

While `visionbridge.settings` triggers state changes, SQLite is the sole database backend used for persistence and telemetry.

- **Location**: Default path `/opt/VLX_VisionBridge/var/visionbridge.db`.
- **Usage**: Stores reusable source templates, layout presets, and broadcast logs (uptime, bitrate fluctuations, error states).

## Security

- Non-root execution constraints are explicitly enforced (except during initial installation).
- `SanitizeInputPath` prevents argument injection (e.g., ensuring paths starting with `-` are prefixed with `./`).
- Strict integer typing for overlay properties (Width, Height, X, Y) prevents filter injection vulnerabilities.
