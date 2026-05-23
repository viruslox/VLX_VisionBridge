1. **Add `InputSettings` struct in `internal/models/models.go`**
   - Add a struct `InputSettings` with a single field `Resolution string \`yaml:"resolution"\``.
   - Add a field `Input InputSettings \`yaml:"input"\`` to the `Config` struct.
2. **Update `configs/visionbridge.settings.template`**
   - Add the `input:` section right after `output:` and before `layers:`.
   - The template should contain `input:\n  resolution: "1920x1080" # Layer's canvas size`.
3. **Update `README.md`**
   - Update the configuration section in README.md or add it where the output is mentioned, referring to the input resolution canvas size.
4. **Update `internal/engine/mixer/mixer.go` to use `cfg.Input.Resolution` for the canvas size**
   - In `BuildFilterComplex`, change `filterComplex.WriteString(cfg.Output.Resolution)` to `filterComplex.WriteString(cfg.Input.Resolution)`.
5. **Update validation in `internal/engine/builder.go`**
   - Currently, it validates `cfg.Output.Resolution`. We probably should add similar validation for `cfg.Input.Resolution` if we're using it for the canvas size (or update the logic if it needs both).
6. **Update config parsing tests (`internal/config/config_test.go` and `internal/engine/builder_test.go`)**
   - Ensure the new `Input` field is set correctly in existing dummy `Config`s in tests so they don't break.
7. **Complete pre-commit steps to ensure proper testing, verification, review, and reflection are done.**
8. **Submit the change.**
