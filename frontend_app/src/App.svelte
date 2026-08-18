<script>
  import { onMount, onDestroy } from "svelte";
  import { getStatus, toggleOutput, toggleLayer, setVolume, shutdown } from "./lib/api.js";
  import Console from "./lib/Console.svelte";
  import Templates from "./lib/Templates.svelte";

  let status = $state({
    output: { active: false, resolution: "", fps: 0 },
    chromium: { active: false },
    overlay_server_active: false,
    resolution: "",
    framerate: 0,
    layers: [],
  });
  let error = $state("");
  let busy = $state(false);
  let tab = $state("control"); // "control" | "templates"
  let poll;

  // local edit buffers so typing a path / dragging volume doesn't fight the poll
  let pathEdit = $state({});

  async function refresh() {
    try {
      status = await getStatus();
      error = "";
    } catch (e) {
      error = e.message || String(e);
    }
  }

  onMount(() => {
    refresh();
    poll = setInterval(refresh, 3000);
  });
  onDestroy(() => clearInterval(poll));

  async function onOutput() {
    busy = true;
    try {
      await toggleOutput(!status.output.active);
      await refresh();
    } catch (e) {
      error = e.message || String(e);
    }
    busy = false;
  }

  async function onLayerToggle(layer) {
    busy = true;
    try {
      await toggleLayer(layer.index, !layer.active);
      await refresh();
    } catch (e) {
      error = e.message || String(e);
    }
    busy = false;
  }

  async function onLayerPath(layer) {
    const path = pathEdit[layer.index];
    if (path === undefined) return;
    busy = true;
    try {
      // enabling with an explicit path sets it; keep current active state
      await toggleLayer(layer.index, layer.active, path);
      delete pathEdit[layer.index];
      await refresh();
    } catch (e) {
      error = e.message || String(e);
    }
    busy = false;
  }

  async function onVolume(layer, value) {
    try {
      await setVolume(layer.index, Number(value));
    } catch (e) {
      error = e.message || String(e);
    }
  }

  async function onShutdown() {
    if (!confirm("Restart the VisionBridge service now?")) return;
    busy = true;
    try {
      await shutdown();
      error = "";
    } catch (e) {
      error = e.message || String(e);
    }
    busy = false;
  }
</script>

<main>
  <h1>VLX VisionBridge</h1>

  <nav class="tabs">
    <button class:sel={tab === "control"} onclick={() => (tab = "control")}>Control</button>
    <button class:sel={tab === "templates"} onclick={() => (tab = "templates")}>Templates</button>
  </nav>

  {#if error}
    <div class="banner err">{error}</div>
  {/if}

  {#if tab === "control"}
    <section class="card">
      <h2>Output</h2>
      <div class="row">
        <span class="dot" class:on={status.output.active}></span>
        <span class="name">Stream</span>
        <span class="meta">{status.output.resolution} @ {status.output.fps}fps</span>
        <button class="toggle" class:active={status.output.active} disabled={busy} onclick={onOutput}>
          {status.output.active ? "ON" : "OFF"}
        </button>
      </div>
      <div class="row">
        <span class="dot" class:on={status.chromium.active}></span>
        <span class="name">Chromium compositor</span>
        <span class="meta">
          canvas {status.resolution} @ {status.framerate}fps · overlay server {status.overlay_server_active ? "up" : "down"}
        </span>
      </div>
    </section>

    <section class="card">
      <h2>Z-Layers</h2>
      <div class="layers">
        {#each status.layers as layer (layer.index)}
          <div class="layer">
            <button
              class="toggle mini"
              class:active={layer.active}
              disabled={busy}
              onclick={() => onLayerToggle(layer)}
              title={layer.active ? "active" : "inactive"}
            >
              Z{layer.index}
            </button>
            <input
              class="path"
              type="text"
              value={pathEdit[layer.index] ?? layer.path}
              oninput={(e) => (pathEdit[layer.index] = e.target.value)}
              onblur={() => onLayerPath(layer)}
              onkeydown={(e) => e.key === "Enter" && onLayerPath(layer)}
              placeholder="path / url"
            />
            <input
              class="vol"
              type="range"
              min="0"
              max="100"
              value={layer.volume}
              oninput={(e) => onVolume(layer, e.target.value)}
            />
            <span class="volval">{layer.volume}</span>
          </div>
        {/each}
      </div>
    </section>

    <section class="card">
      <h2>Console</h2>
      <Console />
    </section>

    <section class="card danger">
      <h2>Service</h2>
      <button class="danger-btn" onclick={onShutdown} disabled={busy}>
        Shutdown (systemd relaunches)
      </button>
    </section>
  {:else}
    <section class="card">
      <h2>Layout Templates</h2>
      <Templates />
    </section>
  {/if}
</main>

<style>
  main {
    max-width: 1000px;
    margin: 0 auto;
    padding: 2rem 1rem;
  }
  h1 {
    text-align: center;
    font-size: 2rem;
  }
  h2 {
    font-size: 1.1rem;
    margin: 0 0 0.75rem;
  }
  .tabs {
    display: flex;
    gap: 0.5rem;
    justify-content: center;
    margin-bottom: 1rem;
  }
  .tabs .sel {
    border-color: #646cff;
    color: #646cff;
  }
  .banner {
    padding: 0.6rem 0.9rem;
    border-radius: 8px;
    margin-bottom: 1rem;
  }
  .banner.err {
    background: rgba(255, 107, 107, 0.15);
    color: #ff8787;
  }
  .card {
    border: 1px solid rgba(128, 128, 128, 0.25);
    border-radius: 10px;
    padding: 1rem 1.25rem;
    margin-bottom: 1rem;
  }
  .card.danger {
    border-color: rgba(255, 107, 107, 0.4);
  }
  .row {
    display: grid;
    grid-template-columns: auto 1fr auto auto;
    align-items: center;
    gap: 0.75rem;
    padding: 0.35rem 0;
    border-bottom: 1px solid rgba(128, 128, 128, 0.12);
  }
  .dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: #888;
  }
  .dot.on {
    background: #37b24d;
  }
  .name {
    font-weight: 500;
  }
  .meta {
    font-size: 0.8rem;
    opacity: 0.65;
  }
  .toggle.active {
    border-color: #37b24d;
    color: #37b24d;
  }
  .toggle.mini {
    min-width: 44px;
  }
  .layers {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .layer {
    display: grid;
    grid-template-columns: auto 1fr 160px auto;
    align-items: center;
    gap: 0.6rem;
  }
  .path {
    padding: 0.3rem 0.5rem;
    border-radius: 6px;
    border: 1px solid rgba(128, 128, 128, 0.35);
    background: transparent;
    color: inherit;
    font-size: 0.85rem;
  }
  .vol {
    width: 160px;
  }
  .volval {
    font-size: 0.8rem;
    opacity: 0.7;
    min-width: 28px;
    text-align: right;
  }
  .danger-btn {
    border-color: rgba(255, 107, 107, 0.5);
    color: #ff8787;
  }
</style>
