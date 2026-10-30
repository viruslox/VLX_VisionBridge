<script>
  import { onMount } from "svelte";
  import { listTemplates, saveTemplate, saveTemplateToFile, applyTemplate, deleteTemplate } from "./api.js";

  let templates = $state([]);
  let newName = $state("");
  let error = $state("");
  let busy = $state(false);

  async function refresh() {
    try {
      templates = await listTemplates();
      error = "";
    } catch (e) {
      error = e.message || String(e);
    }
  }

  onMount(refresh);

  async function onSave() {
    const name = newName.trim();
    if (!name) return;
    busy = true;
    try {
      await saveTemplate(name);
      newName = "";
      await refresh();
    } catch (e) {
      error = e.message || String(e);
    }
    busy = false;
  }

  async function onSaveToFile(name) {
    busy = true;
    try {
      await saveTemplateToFile(name);
      await refresh();
    } catch (e) {
      error = e.message || String(e);
    }
    busy = false;
  }

  async function onApply(name) {
    busy = true;
    try {
      await applyTemplate(name);
      error = "";
    } catch (e) {
      error = e.message || String(e);
    }
    busy = false;
  }

  async function onDelete(name) {
    if (!confirm(`Delete template "${name}"?`)) return;
    busy = true;
    try {
      await deleteTemplate(name);
      await refresh();
    } catch (e) {
      error = e.message || String(e);
    }
    busy = false;
  }
</script>

<div class="templates">
  {#if error}
    <div class="err">{error}</div>
  {/if}

  <div class="save-row">
    <input
      type="text"
      placeholder="Template name"
      bind:value={newName}
      onkeydown={(e) => e.key === "Enter" && onSave()}
    />
    <button onclick={onSave} disabled={busy || !newName.trim()}>Save current layout</button>
  </div>

  {#if templates.length === 0}
    <p class="empty">No templates saved yet. Save the current Z-layout above.</p>
  {:else}
    <ul class="list">
      {#each templates as t (t.name)}
        <li class="item">
          <span class="tname">{t.name}</span>
          <span class="tmeta">{t.updated_at}</span>
          {#if t.has_file}
            <span class="tfile" title="Template is also saved as a .yaml file in the VisionBridge config directory">
              file: {t.file_name || `${t.name}.yaml`}
            </span>
          {:else}
            <button
              class="file-btn"
              onclick={() => onSaveToFile(t.name)}
              disabled={busy}
              title="Export this DB-only template to a .yaml file in the VisionBridge config directory"
            >
              Save to file
            </button>
          {/if}
          <button onclick={() => onApply(t.name)} disabled={busy}>Apply</button>
          <button class="del" onclick={() => onDelete(t.name)} disabled={busy}>Delete</button>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .save-row {
    display: flex;
    gap: 0.5rem;
    margin-bottom: 0.75rem;
  }
  .save-row input {
    flex: 1;
    padding: 0.4rem 0.6rem;
    border-radius: 8px;
    border: 1px solid rgba(128, 128, 128, 0.35);
    background: transparent;
    color: inherit;
  }
  .err {
    color: #ff8787;
    font-size: 0.85rem;
    margin-bottom: 0.5rem;
  }
  .empty {
    opacity: 0.6;
    font-size: 0.9rem;
  }
  .list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .item {
    display: grid;
    grid-template-columns: 1fr auto auto auto auto;
    align-items: center;
    gap: 0.6rem;
    padding: 0.35rem 0;
    border-bottom: 1px solid rgba(128, 128, 128, 0.12);
  }
  .tname {
    font-weight: 500;
  }
  .tmeta {
    font-size: 0.75rem;
    opacity: 0.6;
  }
  .tfile {
    font-size: 0.75rem;
    padding: 0.25rem 0.55rem;
    border-radius: 6px;
    border: 1px solid rgba(55, 178, 77, 0.4);
    color: #37b24d;
  }
  .file-btn {
    border-color: rgba(100, 108, 255, 0.45);
  }
  .del {
    border-color: rgba(255, 107, 107, 0.5);
    color: #ff8787;
  }
</style>
