'use strict';

// ── State ─────────────────────────────────────────────────────────────────────
const flows = new Map();   // id → merged flow object
let selectedId   = null;
let settingsData = null;
let searchFilter = '';

// ── View navigation ───────────────────────────────────────────────────────────
function switchView(name) {
  document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));
  document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));
  document.getElementById('view-' + name).classList.add('active');
  document.getElementById('tab-'  + name).classList.add('active');
  if (name === 'settings') loadSettings();
}

document.getElementById('tab-intercept').addEventListener('click', () => switchView('intercept'));
document.getElementById('tab-inspect')  .addEventListener('click', () => switchView('inspect'));
document.getElementById('tab-settings') .addEventListener('click', () => switchView('settings'));

// ── WebSocket ─────────────────────────────────────────────────────────────────
function connectWS() {
  const ws = new WebSocket(`ws://${location.host}/ws`);
  ws.onmessage = e => {
    let msg; try { msg = JSON.parse(e.data); } catch { return; }
    onMsg(msg);
  };
  ws.onclose = () => setTimeout(connectWS, 1500);
}

function onMsg(msg) {
  switch (msg.type) {
    case 'flow_request': {
      flows.set(msg.id, { ...msg });
      appendFlowItem(msg.id);
      break;
    }
    case 'flow_response':
    case 'flow_paused':
    case 'flow_resumed':
    case 'flow_done': {
      const prev = flows.get(msg.id) || {};
      flows.set(msg.id, { ...prev, ...msg });
      refreshFlowItem(msg.id);
      if (selectedId === msg.id) renderDetail(msg.id);
      break;
    }
    case 'intercept_changed':
      document.getElementById('intercept-cb').checked = msg.enabled;
      break;
  }
}

// ── Flow list ─────────────────────────────────────────────────────────────────
function appendFlowItem(id) {
  const f = flows.get(id);
  if (!matchesFilter(f)) return;

  const list = document.getElementById('flow-list');
  const el = buildFlowItem(id, f);
  list.prepend(el);
}

function refreshFlowItem(id) {
  const f   = flows.get(id);
  const old = document.getElementById('fi-' + id);
  if (!old) {
    // might have been filtered — try inserting
    if (matchesFilter(f)) appendFlowItem(id);
    return;
  }
  const el = buildFlowItem(id, f);
  old.replaceWith(el);
}

function buildFlowItem(id, f) {
  const el = document.createElement('div');
  el.className = 'flow-item' + (id === selectedId ? ' active' : '') + (f.state === 'paused' ? ' paused' : '');
  el.id = 'fi-' + id;
  el.addEventListener('click', () => selectFlow(id));

  const path = f.path || '/';
  const host = f.host || '';

  el.innerHTML = `
    <span class="method-badge mb-${mbClass(f.method)}">${esc(f.method)}</span>
    <span class="flow-item-mid">
      <span class="flow-item-path">${esc(path)}</span>
      <span class="flow-item-host">${esc(host)}</span>
    </span>
    <span class="status-badge ${statusClass(f)}">${statusText(f)}</span>
  `;
  return el;
}

function matchesFilter(f) {
  if (!searchFilter) return true;
  const q = searchFilter.toLowerCase();
  return (f.url || '').toLowerCase().includes(q)
      || (f.host || '').toLowerCase().includes(q)
      || (f.path || '').toLowerCase().includes(q);
}

function refilter() {
  const list = document.getElementById('flow-list');
  list.innerHTML = '';
  // flows are ordered by insertion; prepend re-reverses them so do reverse
  const items = [...flows.entries()].reverse();
  items.forEach(([id, f]) => {
    if (matchesFilter(f)) {
      list.append(buildFlowItem(id, f));
    }
  });
}

document.getElementById('search-input').addEventListener('input', e => {
  searchFilter = e.target.value.trim();
  refilter();
});

// ── Select flow ───────────────────────────────────────────────────────────────
function selectFlow(id) {
  if (selectedId) {
    const prev = document.getElementById('fi-' + selectedId);
    if (prev) prev.classList.remove('active');
  }
  selectedId = id;
  const el = document.getElementById('fi-' + id);
  if (el) el.classList.add('active');

  renderDetail(id);
}

// ── Detail panel ──────────────────────────────────────────────────────────────
async function renderDetail(id) {
  const f = flows.get(id);
  if (!f) return;

  document.getElementById('detail-empty').style.display   = 'none';
  const content = document.getElementById('detail-content');
  content.style.display = 'flex';

  // URL bar
  document.getElementById('detail-url-method').innerHTML =
    `<span class="method-badge mb-${mbClass(f.method)}" style="margin-right:8px">${esc(f.method)}</span>`;
  document.getElementById('detail-url-text').textContent = f.url || (f.host + f.path);

  const statusEl = document.getElementById('detail-url-status');
  statusEl.textContent  = f.status ? String(f.status) : '';
  statusEl.className    = 'detail-url-status ' + (f.status ? 'sc-' + Math.floor(f.status / 100) : 'sc-0');

  // Paused actions
  const actionsEl = document.getElementById('detail-actions');
  actionsEl.innerHTML = '';
  if (f.state === 'paused') {
    const r = document.createElement('button');
    r.className = 'btn btn-green'; r.textContent = '▶ Reanudar';
    r.addEventListener('click', () => resumeFlow(id));
    const b = document.createElement('button');
    b.className = 'btn btn-red'; b.textContent = '✕ Bloquear';
    b.addEventListener('click', () => blockFlow(id));
    actionsEl.append(r, b);
  }

  // Fetch full detail
  try {
    const res  = await fetch(`/api/flows/${id}`);
    const data = await res.json();
    renderReqCol(data);
    renderRespCol(data, f.state);

    // expose data to copy dropdown
    _copyData = data;
    document.getElementById('copy-wrap').style.display = '';
  } catch {
    document.getElementById('req-col').innerHTML  = '<span class="body-empty">Error cargando detalle.</span>';
    document.getElementById('resp-col').innerHTML = '';
  }
}

function renderReqCol(data) {
  const col = document.getElementById('req-col');
  col.innerHTML = '';
  col.appendChild(headersBlock(data.req_headers));
  col.appendChild(bodyBlock(data.req_body));
}

function renderRespCol(data, state) {
  const col = document.getElementById('resp-col');
  col.innerHTML = '';

  if (state === 'paused') {
    const banner = document.createElement('div');
    banner.className = 'paused-banner';
    banner.innerHTML = `
      <div class="paused-banner-title">⏸ Petición en pausa</div>
      <div class="paused-banner-actions">
        <button class="btn btn-green" onclick="resumeFlow('${esc(data.id)}')">▶ Reanudar</button>
        <button class="btn btn-red"   onclick="blockFlow('${esc(data.id)}')">✕ Bloquear</button>
      </div>`;
    col.appendChild(banner);
    return;
  }

  if (state === 'blocked') {
    col.innerHTML = '<span class="body-empty">Petición bloqueada (403).</span>';
    return;
  }

  if (!data.status) {
    col.innerHTML = '<span class="body-empty">Esperando respuesta…</span>';
    return;
  }

  col.appendChild(headersBlock(data.resp_headers));
  col.appendChild(bodyBlock(data.resp_body));
}

function headersBlock(headers) {
  const wrap = document.createElement('div');
  wrap.className = 'headers-list';
  if (!headers || !Object.keys(headers).length) return wrap;
  wrap.innerHTML = `<div class="section-title">Headers</div>` +
    Object.entries(headers).map(([k, v]) =>
      `<div class="header-row"><span class="h-key">${esc(k)}:</span><span class="h-val">${esc(v)}</span></div>`
    ).join('');
  return wrap;
}

function bodyBlock(body) {
  const wrap = document.createElement('div');
  wrap.className = 'body-section';
  if (!body) {
    wrap.innerHTML = '<span class="body-empty">(sin cuerpo)</span>';
    return wrap;
  }
  wrap.innerHTML = `<div class="section-title">Body</div><pre class="body-pre">${esc(prettyJSON(body))}</pre>`;
  return wrap;
}

// ── Intercept (pause all) ─────────────────────────────────────────────────────
document.getElementById('intercept-cb').addEventListener('change', async e => {
  await fetch('/api/intercept', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled: e.target.checked }),
  });
});

async function resumeFlow(id) {
  await fetch(`/api/flows/${id}/resume`, { method: 'POST' });
}
async function blockFlow(id) {
  await fetch(`/api/flows/${id}/block`, { method: 'POST' });
}

// ── Clear ─────────────────────────────────────────────────────────────────────
document.getElementById('clear-btn').addEventListener('click', () => {
  flows.clear();
  selectedId = null;
  _copyData  = null;
  document.getElementById('flow-list').innerHTML = '';
  document.getElementById('detail-empty').style.display   = '';
  document.getElementById('detail-content').style.display = 'none';
  document.getElementById('copy-wrap').style.display      = 'none';
});

// ── JVM grid (Interceptar view) ───────────────────────────────────────────────
async function loadJVMs() {
  const grid = document.getElementById('jvm-grid');
  try {
    const res  = await fetch('/api/jvms');
    const data = await res.json();
    if (!res.ok) {
      grid.innerHTML = `<div class="jvm-empty-state"><span class="big-icon">⚠</span><p>Error: ${esc(data.error || res.status)}</p></div>`;
      return;
    }
    renderJVMGrid(data);
  } catch (e) {
    grid.innerHTML = `<div class="jvm-empty-state"><span class="big-icon">⚠</span><p>Error de conexión:<br>${esc(e.message)}</p></div>`;
  }
}

function renderJVMGrid(jvms) {
  const grid = document.getElementById('jvm-grid');
  if (!jvms || !jvms.length) {
    grid.innerHTML = `<div class="jvm-empty-state"><span class="big-icon">☕</span><p>No se encontraron procesos Java en ejecución.<br>Arranca una app Java e intenta de nuevo.</p></div>`;
    return;
  }
  grid.innerHTML = '';
  jvms.forEach(j => {
    const name = shortName(j.description || j.desc || 'JVM');
    const card = document.createElement('div');
    card.className = 'jvm-card' + (j.attached ? ' attached' : '');
    card.innerHTML = `
      <div class="jvm-card-top">
        <div class="jvm-card-icon">☕</div>
        <div>
          <div class="jvm-card-name">${esc(name)}</div>
          <div class="jvm-card-pid">PID ${esc(String(j.pid))}</div>
        </div>
      </div>
      <div class="jvm-card-desc">${esc(j.description || j.desc || '')}</div>
      ${j.attached
        ? `<button class="jvm-attach-btn attached" disabled>✓ Interceptando</button>`
        : `<button class="jvm-attach-btn" data-pid="${esc(String(j.pid))}">⬤ Adjuntar</button>`}
    `;
    if (!j.attached) {
      card.querySelector('.jvm-attach-btn').addEventListener('click', e => attachJVM(String(j.pid), e.target, card));
    }
    grid.appendChild(card);
  });
}

async function attachJVM(pid, btn, card) {
  btn.disabled    = true;
  btn.textContent = 'Adjuntando…';
  try {
    const res = await fetch(`/api/jvms/${pid}/attach`, { method: 'POST' });
    if (!res.ok) {
      const d = await res.json().catch(() => ({}));
      btn.textContent = '✗ ' + (d.error || 'Error');
      btn.style.color = 'var(--s5)';
      btn.disabled = false;
    } else {
      btn.textContent = '✓ Interceptando';
      btn.className   = 'jvm-attach-btn attached';
      card.classList.add('attached');
      card.querySelector('.jvm-card-icon').style.borderColor = 'var(--teal)';
    }
  } catch (e) {
    btn.textContent = '✗ Error';
    btn.style.color = 'var(--s5)';
    btn.disabled = false;
  }
}

document.getElementById('refresh-btn').addEventListener('click', loadJVMs);

// ── Settings ──────────────────────────────────────────────────────────────────
async function loadSettings() {
  try {
    const res  = await fetch('/api/settings');
    const data = await res.json();
    settingsData = data;

    // Update proxy address everywhere
    const addr = `${data.proxy_host || '127.0.0.1'}:${data.proxy_port || '8080'}`;
    document.getElementById('proxy-addr').textContent        = addr;
    document.getElementById('proxy-banner-addr').textContent = addr;

    // Auto-detection banner
    const banner = document.getElementById('auto-banner');
    if (data.java_auto_detected && data.java_auto_path) {
      banner.style.display = 'flex';
      document.getElementById('auto-banner-path').textContent = data.java_auto_path;
    } else {
      banner.style.display = 'none';
    }

    renderJDKList(data.jdks || [], data.java_bin);
    document.getElementById('java-manual').value = data.java_bin || '';
    document.getElementById('proxy-host').value  = data.proxy_host || '127.0.0.1';
    document.getElementById('proxy-port').value  = data.proxy_port || '8080';
  } catch (e) {
    setStatus('Error al cargar ajustes: ' + e.message, 'var(--s5)');
  }
}

function renderJDKList(jdks, currentBin) {
  const container = document.getElementById('jdk-list');
  if (!jdks.length) {
    container.innerHTML = '<div style="font-size:11px;color:var(--sub)">No se encontraron JDKs instalados.</div>';
    return;
  }
  container.innerHTML = '';
  jdks.forEach((j, i) => {
    const isSelected = j.path === currentBin;
    const isBest     = i === 0;
    const isAuto     = isSelected && settingsData?.java_auto_detected;
    const verText    = j.major ? `Java ${j.major}` : 'versión desconocida';
    const autoNote   = isAuto ? ' — auto' : '';

    const label = document.createElement('label');
    label.className = 'jdk-item' + (isSelected ? ' selected' : '');
    label.innerHTML = `
      <input type="radio" name="jdk" value="${esc(j.path)}" ${isSelected ? 'checked' : ''}>
      <div class="jdk-item-info">
        <div class="jdk-item-name">${esc(j.name)}</div>
        <div class="jdk-item-ver">${verText}${autoNote} · ${esc(j.path)}</div>
      </div>
      ${isBest ? '<span class="badge badge-rec">Recomendado</span>' : ''}
      ${isAuto  ? '<span class="badge badge-auto">Auto</span>'      : ''}
    `;
    label.querySelector('input').addEventListener('change', () => {
      container.querySelectorAll('.jdk-item').forEach(x => x.classList.remove('selected'));
      label.classList.add('selected');
      document.getElementById('java-manual').value = j.path;
    });
    container.appendChild(label);
  });
}

document.getElementById('settings-save').addEventListener('click', async () => {
  const javaBin = document.getElementById('java-manual').value.trim()
    || document.querySelector('input[name="jdk"]:checked')?.value || '';
  const proxyHost = document.getElementById('proxy-host').value.trim();
  const proxyPort = document.getElementById('proxy-port').value.trim();

  if (!javaBin) { setStatus('Selecciona un JDK o escribe una ruta.', 'var(--s3)'); return; }

  setStatus('Guardando…', 'var(--sub)');
  try {
    const res = await fetch('/api/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ java_bin: javaBin, proxy_host: proxyHost, proxy_port: proxyPort }),
    });
    if (!res.ok) { const d = await res.json().catch(() => ({})); throw new Error(d.error || res.statusText); }

    const addr = `${proxyHost}:${proxyPort}`;
    document.getElementById('proxy-addr').textContent        = addr;
    document.getElementById('proxy-banner-addr').textContent = addr;
    document.getElementById('auto-banner').style.display = 'none';
    if (settingsData) settingsData.java_auto_detected = false;

    setStatus('✓ Guardado', 'var(--s2)');
    setTimeout(() => setStatus('', ''), 3000);
  } catch (e) {
    setStatus('Error: ' + e.message, 'var(--s5)');
  }
});

function setStatus(msg, color) {
  const el = document.getElementById('settings-status');
  el.textContent = msg; el.style.color = color || '';
}

// ── Load history on startup ───────────────────────────────────────────────────
async function loadFlows() {
  try {
    const res  = await fetch('/api/flows');
    const data = await res.json();
    if (!Array.isArray(data)) return;
    // server returns oldest→newest; prepend reverses, so push in order and let list display
    data.forEach(f => flows.set(f.id, f));
    refilter();
  } catch {}
}

// ── Copy-as formatters ────────────────────────────────────────────────────────
const SKIP_HDR = new Set([
  'proxy-connection','proxy-authorization','proxy-authenticate',
  'transfer-encoding','content-length','host',
]);

function fmtCurl(d) {
  const parts = [`curl -X ${d.method} '${d.url}'`];
  for (const [k, v] of Object.entries(d.req_headers || {})) {
    if (SKIP_HDR.has(k.toLowerCase())) continue;
    parts.push(`  -H '${k}: ${v.replace(/\\/g,'\\\\').replace(/'/g,"'\\''")}'`);
  }
  if (d.req_body) {
    parts.push(`  --data-raw '${d.req_body.replace(/'/g,"'\\''")}'`);
  }
  return parts.join(' \\\n');
}

function fmtFetch(d) {
  const headers = filteredHeaders(d.req_headers);
  const lines = [`fetch(${JSON.stringify(d.url)}, {`];
  lines.push(`  method: ${JSON.stringify(d.method)},`);
  if (Object.keys(headers).length) {
    lines.push(`  headers: ${JSON.stringify(headers, null, 2).replace(/\n/g,'\n  ')},`);
  }
  if (d.req_body) {
    const isJSON = isJSONContent(d.req_headers);
    lines.push(`  body: ${isJSON ? 'JSON.stringify(' + d.req_body + ')' : JSON.stringify(d.req_body)},`);
  }
  lines.push('})');
  return lines.join('\n');
}

function fmtAxios(d) {
  const headers = filteredHeaders(d.req_headers);
  const cfg = { method: d.method.toLowerCase(), url: d.url };
  if (Object.keys(headers).length) cfg.headers = headers;
  if (d.req_body) {
    try { cfg.data = JSON.parse(d.req_body); } catch { cfg.data = d.req_body; }
  }
  return `axios(${JSON.stringify(cfg, null, 2)})`;
}

function fmtPython(d) {
  const headers = filteredHeaders(d.req_headers);
  const url = JSON.stringify(d.url);
  const method = d.method.toLowerCase();
  const lines = ['import requests', '', `response = requests.${method}(`];
  lines.push(`    ${url},`);
  if (Object.keys(headers).length) {
    lines.push(`    headers=${pyDict(headers)},`);
  }
  if (d.req_body) {
    const isJSON = isJSONContent(d.req_headers);
    if (isJSON) {
      try {
        lines.push(`    json=${pyDict(JSON.parse(d.req_body))},`);
      } catch {
        lines.push(`    data=${JSON.stringify(d.req_body)},`);
      }
    } else {
      lines.push(`    data=${JSON.stringify(d.req_body)},`);
    }
  }
  lines.push(')');
  return lines.join('\n');
}

function fmtRaw(d) {
  const u = new URL(d.url);
  const path = u.pathname + (u.search || '');
  const lines = [`${d.method} ${path} HTTP/1.1`, `Host: ${u.host}`];
  for (const [k, v] of Object.entries(d.req_headers || {})) {
    if (k.toLowerCase() === 'host') continue;
    lines.push(`${k}: ${v}`);
  }
  if (d.req_body) lines.push('', d.req_body);
  return lines.join('\r\n');
}

// helpers for formatters
function filteredHeaders(h) {
  const out = {};
  for (const [k, v] of Object.entries(h || {})) {
    if (!SKIP_HDR.has(k.toLowerCase())) out[k] = v;
  }
  return out;
}

function isJSONContent(headers) {
  const ct = (headers || {})['Content-Type'] || (headers || {})['content-type'] || '';
  return ct.includes('json');
}

function pyDict(obj) {
  const inner = Object.entries(obj)
    .map(([k,v]) => `    ${JSON.stringify(k)}: ${JSON.stringify(v)}`)
    .join(',\n');
  return `{\n${inner}\n}`;
}

// ── Export modal ──────────────────────────────────────────────────────────────
const FORMATS = {
  curl:   { fn: fmtCurl,   label: 'cURL' },
  fetch:  { fn: fmtFetch,  label: 'fetch' },
  axios:  { fn: fmtAxios,  label: 'axios' },
  python: { fn: fmtPython, label: 'Python requests' },
  http:   { fn: fmtRaw,    label: 'HTTP raw' },
};

let _copyData  = null;
let _activeFmt = 'curl';

function setupExportModal() {
  const overlay  = document.getElementById('export-overlay');
  const closeBtn = document.getElementById('export-close');
  const copyBtn  = document.getElementById('export-copy-btn');
  const openBtn  = document.getElementById('copy-btn');

  // Open modal
  openBtn.addEventListener('click', () => {
    if (!_copyData) return;
    document.getElementById('export-url').textContent =
      (_copyData.method || '') + ' ' + (_copyData.url || '');
    overlay.classList.add('open');
    renderExportCode(_activeFmt);
  });

  // Tab switching
  document.getElementById('export-tabs').addEventListener('click', e => {
    const tab = e.target.closest('.export-tab');
    if (!tab) return;
    _activeFmt = tab.dataset.fmt;
    document.querySelectorAll('.export-tab').forEach(t => t.classList.remove('active'));
    tab.classList.add('active');
    renderExportCode(_activeFmt);
  });

  // Copy button
  copyBtn.addEventListener('click', () => {
    if (!_copyData) return;
    const text = FORMATS[_activeFmt].fn(_copyData);
    navigator.clipboard.writeText(text).then(() => {
      const el = document.getElementById('export-copied');
      el.textContent = '✓ Copiado';
      setTimeout(() => { el.textContent = ''; }, 2000);
    });
  });

  // Close on overlay click or close button
  closeBtn.addEventListener('click', closeExportModal);
  overlay.addEventListener('click', e => { if (e.target === overlay) closeExportModal(); });
  document.addEventListener('keydown', e => { if (e.key === 'Escape') closeExportModal(); });
}

function closeExportModal() {
  document.getElementById('export-overlay').classList.remove('open');
}

function renderExportCode(fmt) {
  if (!_copyData || !FORMATS[fmt]) return;
  try {
    document.getElementById('export-code').textContent = FORMATS[fmt].fn(_copyData);
  } catch (e) {
    document.getElementById('export-code').textContent = 'Error generando formato: ' + e.message;
  }
}

function showToast(msg) {
  const old = document.querySelector('.toast');
  if (old) old.remove();
  const t = document.createElement('div');
  t.className = 'toast';
  t.textContent = msg;
  document.body.appendChild(t);
  setTimeout(() => t.remove(), 2300);
}

// ── Helpers ───────────────────────────────────────────────────────────────────
function esc(s) {
  return String(s ?? '').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

function mbClass(m) {
  const map = { GET:'get', POST:'post', PUT:'put', PATCH:'patch', DELETE:'delete' };
  return map[(m||'').toUpperCase()] || 'other';
}

function statusClass(f) {
  if (f.state === 'paused')  return 'sc-0';
  if (f.state === 'blocked') return 'sc-5';
  if (!f.status) return 'sc-0';
  return 'sc-' + Math.floor(f.status / 100);
}

function statusText(f) {
  if (f.state === 'paused')  return '⏸';
  if (f.state === 'blocked') return '✕';
  if (!f.status) return '…';
  return String(f.status);
}

function prettyJSON(s) {
  try { return JSON.stringify(JSON.parse(s), null, 2); } catch { return s; }
}

function shortName(desc) {
  // "com.example.MyApp --args" → "MyApp"
  const parts = (desc || '').split(/\s+/)[0].split('.');
  return parts[parts.length - 1] || desc;
}

// ── Boot ──────────────────────────────────────────────────────────────────────
setupExportModal();
connectWS();
loadJVMs();
loadFlows();
setInterval(loadJVMs, 5000);
