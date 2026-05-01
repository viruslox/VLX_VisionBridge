1. We need to fix the `BuildFFmpegArgs` logic in `internal/engine/builder.go`.
2. Issue 1: Input indexing mismatch. We need to track the `inputIdx` explicitly, incrementing it only when an active layer is processed. So `[inputIdx:v]` instead of `[i:v]`.
3. Issue 2: Broken Filter Chaining. We need to track the `prevOutPad` (e.g. tracking `lastOutPadName`) and update it after each overlay, instead of hardcoding `[out{i-1}]`.
4. Issue 3: Missing base canvas. We should unconditionally create the base canvas `[base]` with the configured resolution and color black at the beginning if there are any active layers. Then all active layers are overlaid onto the chain starting with `[base]`.

Let's implement these changes.
