# Proposed Plan

## Context
The user requested:
1. Verify phase 1 completion and update PLAN.md.
2. Verify phase 2 completion and update PLAN.md.
3. Fully implement phase 3 and update PLAN.md.

### Phase 1 checklist from PLAN.md:
- [ ] Initialize Go module `github.com/user/go-live-orchestrator`.
- [ ] Setup Directory Structure: `cmd/server/main.go`, `internal/config/`, `internal/engine/`, `internal/db/`, `internal/models/`, `configs/config.yaml`, `scripts/`
- [ ] Implement basic PostgreSQL connection pool.

**Status**:
- `go.mod` is initialized.
- Directories mostly exist, I just created `internal/engine` and `internal/models`. I'll also create `scripts` directory.
- `internal/db/db.go` exists but has a placeholder: `// Implement basic PostgreSQL connection pool here.`. I need to implement a real pgx/pg pool.

### Phase 2 checklist from PLAN.md:
- [x] Create `config.yaml` parser.
- [x] Implement `fsnotify` to detect file changes.
- [x] Logic for "Diffing": Determine if a change requires a full FFmpeg restart or just a filter update.

**Status**:
- `internal/config/config.go` has everything implemented. So Phase 2 is complete.

### Phase 3 checklist from PLAN.md:
- [ ] Develop the logic to convert "Layer 0-9" structs into an FFmpeg `filter_complex` string.
- [ ] Implement coordinate calculation (handling 'center', 'top-left', and 5% padding).
- [ ] Handle input types: `folder` (image sequence), `loop` (video), `SRT`.

**Status**:
- Nothing is implemented yet. Needs to be built in `internal/engine/ffmpeg.go` or similar. We'll need a way to parse the `config.Layer` structures and generate `-filter_complex` arguments, plus `-i` input strings based on the input type. We also need to extract `config.Layer` into `internal/models` to adhere to the Phase 1 directory structure, or keep it in config and refer to it. The prompt hints at `internal/models/models.go` or similar. Wait, the structure says `models/ # Layer & Stream structs`. So I will move `Layer` and `OutputSettings` from `config.go` to `models` or create them there and update `config.go`. Wait, maybe just leave it in `config.go` for now or better, move it to `internal/models` to fulfill Phase 1 directory structure.

## Plan Steps

1. **Complete Phase 1**:
    - Implement PostgreSQL connection pool in `internal/db/db.go` using `database/sql` and `github.com/lib/pq` or `github.com/jackc/pgx`. Let's use standard `database/sql` and `github.com/lib/pq` (add to go.mod).
    - Create `scripts` directory.
    - Check off Phase 1 in `PLAN.md`.

2. **Verify Phase 2**:
    - Phase 2 is already implemented (`internal/config/config.go`), but the plan has it already checked off. Ensure it's checked off in `PLAN.md` (it is).

3. **Implement Phase 3**:
    - Create `internal/engine/ffmpeg.go`.
    - Implement logic to map inputs (`folder`, `loop`, `srt`).
    - Implement logic to generate `filter_complex` string for coordinates (`center`, `top-left`, with 5% padding), handling active/inactive state, scale, and crop.
    - Write unit tests for this filter complex generation to ensure correctness.
    - Check off Phase 3 in `PLAN.md`.

4. **Refactor (Optional but good for Phase 1 structure)**:
    - Move `Layer` and `Config` models to `internal/models/` if strictly required, but it's simpler to keep it where it is or use `config.Layer` directly. Actually, the phase 1 says `internal/models/ # Layer & Stream structs`. Let's move them to `internal/models/models.go` and update `internal/config/config.go`.

5. **Pre-commit Steps**:
    - Run `pre_commit_instructions` tool to verify before submission. Ensure tests pass and the code is formatted.

6. **Submit Code**:
    - Submit branch.
