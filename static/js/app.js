// Petit wrapper AJAX commun basé sur fetch(), utilisé par toutes les pages.
async function apiGet(url) {
  const res = await fetch(url, { credentials: "same-origin" });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Erreur HTTP ${res.status}`);
  return data;
}

async function apiSend(url, method, body) {
  const res = await fetch(url, {
    method,
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: body ? JSON.stringify(body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Erreur HTTP ${res.status}`);
  return data;
}

async function apiPost(url, body) { return apiSend(url, "POST", body); }
async function apiPut(url, body) { return apiSend(url, "PUT", body); }
async function apiDelete(url) { return apiSend(url, "DELETE"); }

async function apiUpload(url, formData) {
  const res = await fetch(url, { method: "POST", credentials: "same-origin", body: formData });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error((data.error && data.error.message) || data.error || `Erreur HTTP ${res.status}`);
  return data;
}

function fmtDate(iso) {
  try { return new Date(iso).toLocaleString("fr-FR"); } catch (e) { return iso; }
}

function showError(el, err) {
  el.textContent = err.message || String(err);
  el.style.display = "block";
}
