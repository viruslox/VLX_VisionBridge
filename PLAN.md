# Project Implementation Plan

## Phase 1: Environment & Project Scaffolding
- [ ] Initialize Go module `github.com/user/go-live-orchestrator`.
- [ ] Setup Directory Structure:
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
- [ ] Implement basic PostgreSQL connection pool.

## Phase 2: Configuration & Hot-Reloading
- [ ] Create `config.yaml` parser.
- [ ] Implement `fsnotify` to detect file changes.
- [ ] Logic for "Diffing": Determine if a change requires a full FFmpeg restart or just a filter update.

## Phase 3: The FFmpeg Wrapper (The Core)
- [ ] Develop the logic to convert "Layer 0-9" structs into an FFmpeg `filter_complex` string.
- [ ] Implement coordinate calculation (handling 'center', 'top-left', and 5% padding).
- [ ] Handle input types: `folder` (image sequence), `loop` (video), `SRT`.

## Phase 4: Output & Multi-Streaming
- [ ] Implement the `tee` muxer logic for simultaneous output to Twitch, YouTube, and VK.
- [ ] Add global output settings (FPS, Bitrate, Resolution).

## Phase 5: Monitoring & Stability
- [ ] Implement automatic process recovery (if FFmpeg crashes, Go restarts it).
- [ ] Logging of stream events to PostgreSQL.
- [ ] Graceful shutdown handling.

## Phase 6: Testing & Optimization
- [ ] Stress test with 10 concurrent SRT inputs.
- [ ] CPU profiling to ensure "OFF" sources consume 0% resources.
- [ ] Network latency optimization for SRT.