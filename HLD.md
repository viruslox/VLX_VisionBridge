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
