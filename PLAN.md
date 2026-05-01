# Project Implementation Plan

## Phase 1: Environment & Project Scaffolding

- [x] Initialize Go module `github.com/user/go-live-orchestrator`.
- [x] Setup Directory Structure:

    ```text
    ├── cmd/server/main.go
    ├── internal/
    │   ├── config/      # File watcher & Parser
    │   ├── engine/      # FFmpeg command generator
    │   ├── db/          # Postgres migrations & queries
    │   └── models/      # Layer & Stream structs
    ├── configs/
    │   └── config.yaml
    └── scripts/         # Deployment & Systemd setup
    ```

- [x] Implement basic PostgreSQL connection pool.

## Phase 2: Configuration & Hot-Reloading

- [x] Create `config.yaml` parser.
- [x] Implement `fsnotify` to detect file changes.
- [x] Logic for "Diffing": Determine if a change requires a full FFmpeg restart or just a filter update.

## Phase 3: The FFmpeg Wrapper (The Core)

- [x] Develop the logic to convert "Layer 0-9" structs into an FFmpeg `filter_complex` string.
- [x] Implement coordinate calculation (handling 'center', 'top-left', and 5% padding).
- [x] Handle input types: `folder` (image sequence), `loop` (video), `SRT`.
- [x] Fix input indexing mismatch (`[inputIdx:v]`).
- [x] Fix broken filter chaining (tracking `prevOutPad`).
- [x] Add base canvas creation logic.
- [x] Move `Layer` and `Config` models to `internal/models/`.

## Phase 4: Output & Multi-Streaming

- [x] Implement the `tee` muxer logic for simultaneous output to Twitch, YouTube, and VK.
- [x] Add global output settings (FPS, Bitrate, Resolution).

## Phase 5: Monitoring & Stability

- [x] Implement automatic process recovery (if FFmpeg crashes, Go restarts it).
- [x] Logging of stream events to PostgreSQL.
- [x] Graceful shutdown handling.

## Phase 6: Testing & Optimization

- [x] Stress test with 10 concurrent SRT inputs.
- [x] CPU profiling to ensure "OFF" sources consume 0% resources.
- [x] Network latency optimization for SRT.

## Phase 7: Build & Setup

- [x] Create build script that creates execuatable and configuration files templates
- [x] Configuration file(s) template must offer all available options (each one briefly explained).
- [x] Normal user have to be able of build, install, execute the application. (no root user allowed).
- [x] Installer asks user for installation path (that will contains executable(s) and config
- [x] build script and executable to inform the user if any pre-requisite/3rd part is missing.
