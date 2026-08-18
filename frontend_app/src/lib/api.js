// Relative URLs so the SPA works behind an Apache subpath in front of the
// frontend server (Vite base "./").

export async function getStatus() {
  const r = await fetch("api/status");
  if (!r.ok) throw new Error("status request failed: " + r.status);
  return r.json();
}

export function toggleOutput(enabled) {
  return post("api/output", { enabled });
}

export function toggleLayer(index, enabled, path) {
  const body = { index, enabled };
  if (path !== undefined && path !== null) body.path = path;
  return post("api/layer", body);
}

export function setVolume(index, volume) {
  return post("api/volume", { index, volume });
}

export function shutdown() {
  return post("api/shutdown", {});
}

export async function consoleTicket() {
  const r = await fetch("api/console/ticket");
  if (!r.ok) throw new Error("console ticket failed: " + r.status);
  return (await r.json()).ticket;
}

export async function listTemplates() {
  const r = await fetch("api/templates");
  if (!r.ok) throw new Error("templates list failed: " + r.status);
  return (await r.json()).templates || [];
}

export function saveTemplate(name) {
  return post("api/templates/save", { name });
}
export function applyTemplate(name) {
  return post("api/templates/apply", { name });
}
export function deleteTemplate(name) {
  return post("api/templates/delete", { name });
}

async function post(url, body) {
  const r = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!r.ok) {
    let msg = "request failed: " + r.status;
    try {
      const e = await r.json();
      if (e && e.error) msg = e.error;
    } catch (_) {
      /* ignore */
    }
    throw new Error(msg);
  }
  return r.json();
}
