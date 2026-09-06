# VLX VisionBridge API & IPC Contract

> **Part of the VLX Stream Flow ecosystem — Composition tier.**

VLX VisionBridge acts as the headless video compositor. It accepts commands via a local Unix socket (`/tmp/vlx_control.sock`), which are usually issued by **ChatBridge** via the `ipc` transport.

---

## The IPC Contract

The payload sent over the Unix socket is newline-delimited JSON. When constructing `.json` commands in ChatBridge, ChatBridge wraps them automatically. You only need to know the `action`, `target`, and `payload` structures below.

### 1. Scene Switching (`apply_template`)

Loads a new Z-order layout template into VisionBridge, effectively changing the scene (e.g. from Camera 1 to Camera 2). The template file must exist in the VisionBridge configuration directory alongside the active settings file.

**ChatBridge Action Structure:**
```json
{
  "transport": "ipc",
  "action": "apply_template",
  "payload": {
    "text": "camera1_layout.yaml"
  }
}
```

### 2. Stream Egress Control (`set_input_state` → `stream`)

Enables or disables the final output (restream) from VisionBridge. Disabling it triggers an immediate `SIGKILL` to the egress FFmpeg process.

**ChatBridge Action Structure:**
```json
{
  "transport": "ipc",
  "action": "set_input_state",
  "target": "stream",
  "payload": {
    "enabled": true
  }
}
```

### 3. Z-Layer Overlays (`set_input_state` → `overlay@layerN`)

Toggles the visibility of a specific Chromium Z-layer (where `layer1` maps to `z1` in VisionBridge config). Optionally, you can pass a `text` payload to update the URL/path of the layer when enabling it.

**ChatBridge Action Structure:**
```json
{
  "transport": "ipc",
  "action": "set_input_state",
  "target": "overlay@layer2",
  "payload": {
    "enabled": true,
    "text": "http://127.0.0.1:8000/gps_overlay.html"
  }
}
```

### 4. Z-Layer Volume (`set_input_state` → `volume@layerN`)

Adjusts the audio volume of a specific Chromium Z-layer in real time without restarting the layer.

**ChatBridge Action Structure:**
```json
{
  "transport": "ipc",
  "action": "set_input_state",
  "target": "volume@layer3",
  "payload": {
    "text": "75"
  }
}
```

### 5. Chromium Restart (`reload` → `chromium`)

Forces the headless Chromium DOM engine to restart entirely. Useful as a panic button if an overlay gets stuck.

**ChatBridge Action Structure:**
```json
{
  "transport": "ipc",
  "action": "reload",
  "target": "chromium"
}
```
