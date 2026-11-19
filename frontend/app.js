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
document.getElementById('tab-rules')    .addEventListener('click', () => { switchView('rules'); loadRules(); });
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

    // expose data to copy dropdown / rule button
    _copyData = data;
    document.getElementById('copy-wrap').style.display = '';
    document.getElementById('rule-wrap').style.display = '';
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
    col.appendChild(buildPausedEditor(data));
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

function buildPausedEditor(data) {
  const banner = document.createElement('div');
  banner.className = 'paused-banner';

  const title = document.createElement('div');
  title.className = 'paused-banner-title';
  title.textContent = '⏸ Petición en pausa — edita y reanuda';
  banner.appendChild(title);

  // Headers
  const hdrLabel = document.createElement('div');
  hdrLabel.className = 'paused-edit-label';
  const hdrAddBtn = document.createElement('button');
  hdrAddBtn.className = 'btn hdr-add-btn';
  hdrAddBtn.textContent = '+ Header';
  hdrAddBtn.addEventListener('click', () => addHeaderRow('intercept-hdr-rows', '', ''));
  hdrLabel.append(Object.assign(document.createElement('span'), {textContent: 'Headers'}), hdrAddBtn);
  banner.appendChild(hdrLabel);

  const hdrRows = document.createElement('div');
  hdrRows.className = 'hdr-rows';
  hdrRows.id = 'intercept-hdr-rows';
  banner.appendChild(hdrRows);
  for (const [k, v] of Object.entries(data.req_headers || {})) {
    addHeaderRow('intercept-hdr-rows', k, v);
  }

  // Body
  const bodyLabel = document.createElement('div');
  bodyLabel.className = 'paused-edit-label';
  bodyLabel.append(Object.assign(document.createElement('span'), {textContent: 'Body'}));
  banner.appendChild(bodyLabel);

  const bodyTA = document.createElement('textarea');
  bodyTA.className = 'intercept-body-ta';
  bodyTA.id = 'intercept-body';
  bodyTA.value = data.req_body || '';
  banner.appendChild(bodyTA);

  // Actions
  const actions = document.createElement('div');
  actions.className = 'paused-banner-actions';
  actions.style.marginTop = '10px';

  const withBtn = document.createElement('button');
  withBtn.className = 'btn btn-green';
  withBtn.textContent = '▶ Reanudar con cambios';
  withBtn.addEventListener('click', () => resumeFlowWith(data.id));

  const plainBtn = document.createElement('button');
  plainBtn.className = 'btn';
  plainBtn.textContent = '▶ Reanudar';
  plainBtn.addEventListener('click', () => resumeFlow(data.id));

  const blockBtn = document.createElement('button');
  blockBtn.className = 'btn btn-red';
  blockBtn.textContent = '✕ Bloquear';
  blockBtn.addEventListener('click', () => blockFlow(data.id));

  actions.append(withBtn, plainBtn, blockBtn);
  banner.appendChild(actions);
  return banner;
}

async function resumeFlowWith(id) {
  const headers = readHeaderRows('intercept-hdr-rows') || {};
  const body = document.getElementById('intercept-body').value;
  await fetch(`/api/flows/${id}/resume`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ headers, body }),
  });
}

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
  document.getElementById('rule-wrap').style.display      = 'none';
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

// ── Rules ─────────────────────────────────────────────────────────────────────
let _editingRuleId = null;

async function loadRules() {
  try {
    const res   = await fetch('/api/rules');
    const rules = await res.json();
    renderRulesList(Array.isArray(rules) ? rules : []);
  } catch (e) {
    document.getElementById('rules-list').innerHTML =
      `<div class="rules-empty"><p style="color:var(--s5)">Error cargando reglas: ${esc(e.message)}</p></div>`;
  }
}

function renderRulesList(rules) {
  const list  = document.getElementById('rules-list');
  const empty = document.getElementById('rules-empty');
  list.innerHTML = '';
  if (!rules.length) {
    list.appendChild(empty || (() => {
      const d = document.createElement('div');
      d.id = 'rules-empty';
      d.className = 'rules-empty';
      d.innerHTML = '<span class="big-icon">⚡</span><p>Sin reglas. Crea una para bloquear, mockear o modificar respuestas automáticamente.</p>';
      return d;
    })());
    return;
  }
  rules.forEach(r => list.appendChild(buildRuleItem(r)));
}

function buildRuleItem(r) {
  const item = document.createElement('div');
  item.className = 'rule-item' + (r.enabled ? '' : ' disabled');
  item.dataset.id = r.id;

  const badgeClass = { block: 'ab-block', mock: 'ab-mock', modify_headers: 'ab-modify', replace_body: 'ab-replace' }[r.action.type] || 'ab-block';
  const badgeLabel = { block: 'Block', mock: 'Mock', modify_headers: 'Mod-Hdr', replace_body: 'Repl-Body' }[r.action.type] || r.action.type;

  item.innerHTML = `
    <label class="rule-toggle" title="${r.enabled ? 'Desactivar' : 'Activar'}">
      <input type="checkbox" ${r.enabled ? 'checked' : ''}>
      <span class="rule-toggle-track"></span>
      <span class="rule-toggle-thumb"></span>
    </label>
    <div class="rule-info">
      <div class="rule-name">${esc(r.name || '(sin nombre)')}</div>
      <div class="rule-cond">${esc(conditionSummary(r.condition))}</div>
    </div>
    <span class="action-badge ${badgeClass}">${badgeLabel}</span>
    <button class="btn" data-edit="${esc(r.id)}" title="Editar">✎</button>
    <button class="btn btn-red" data-del="${esc(r.id)}" title="Eliminar">✕</button>
  `;

  item.querySelector('input[type=checkbox]').addEventListener('change', async e => {
    const updated = { ...r, enabled: e.target.checked };
    await fetch(`/api/rules/${r.id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(updated),
    });
    item.classList.toggle('disabled', !e.target.checked);
  });

  item.querySelector('[data-edit]').addEventListener('click', () => openRuleModal(r));
  item.querySelector('[data-del]').addEventListener('click', async () => {
    if (!confirm(`¿Eliminar la regla "${r.name}"?`)) return;
    await fetch(`/api/rules/${r.id}`, { method: 'DELETE' });
    loadRules();
  });

  return item;
}

function conditionSummary(c) {
  if (!c) return '*';
  const parts = [];
  if (c.method)      parts.push(c.method);
  if (c.url_pattern) parts.push('~ ' + c.url_pattern);
  else if (c.host)   parts.push(c.host);
  if (c.header_name) parts.push(`hdr:${c.header_name}`);
  if (c.status_code) parts.push(`status:${c.status_code}`);
  return parts.join(' ') || '*';
}

// ── Rule modal helpers ────────────────────────────────────────────────────────
const CT_KNOWN = [
  'application/json','application/xml','text/html','text/plain',
  'application/x-www-form-urlencoded','application/octet-stream',
];

function getCT(prefix) {
  const sel = document.getElementById(`rule-${prefix}-ct-sel`);
  if (sel.value === '__custom__') {
    return document.getElementById(`rule-${prefix}-ct-custom`).value.trim();
  }
  return sel.value;
}

function setCT(prefix, value) {
  const sel    = document.getElementById(`rule-${prefix}-ct-sel`);
  const custom = document.getElementById(`rule-${prefix}-ct-custom`);
  const v = value || 'application/json';
  if (CT_KNOWN.includes(v)) {
    sel.value = v;
    custom.style.display = 'none';
  } else {
    sel.value    = '__custom__';
    custom.value = v;
    custom.style.display = '';
  }
}

function wireCTSel(prefix) {
  document.getElementById(`rule-${prefix}-ct-sel`).addEventListener('change', function() {
    document.getElementById(`rule-${prefix}-ct-custom`).style.display =
      this.value === '__custom__' ? '' : 'none';
  });
}

function addHeaderRow(containerId, key, val) {
  const container = document.getElementById(containerId);
  const row = document.createElement('div');
  row.className = 'hdr-row';
  row.innerHTML = `
    <input class="settings-input hdr-key" type="text" placeholder="Nombre" value="${esc(key || '')}">
    <input class="settings-input hdr-val" type="text" placeholder="Valor"  value="${esc(val || '')}">
    <button class="btn hdr-rm" title="Eliminar">✕</button>
  `;
  row.querySelector('.hdr-rm').addEventListener('click', () => row.remove());
  container.appendChild(row);
}

function setHeaderRows(containerId, headers) {
  document.getElementById(containerId).innerHTML = '';
  for (const [k, v] of Object.entries(headers || {})) {
    addHeaderRow(containerId, k, v);
  }
}

function readHeaderRows(containerId) {
  const h = {};
  document.querySelectorAll(`#${containerId} .hdr-row`).forEach(row => {
    const k = row.querySelector('.hdr-key').value.trim();
    const v = row.querySelector('.hdr-val').value.trim();
    if (k) h[k] = v;
  });
  return Object.keys(h).length ? h : undefined;
}

wireCTSel('mock');
wireCTSel('replace');

document.getElementById('rule-mock-hdr-add').addEventListener('click', () => addHeaderRow('rule-mock-hdr-rows', '', ''));
document.getElementById('rule-mod-hdr-add') .addEventListener('click', () => addHeaderRow('rule-mod-hdr-rows', '', ''));

document.getElementById('rule-mock-fmt')   .addEventListener('click', () => fmtBodyTA('rule-mock-body'));
document.getElementById('rule-replace-fmt').addEventListener('click', () => fmtBodyTA('rule-replace-body'));

function fmtBodyTA(id) {
  const ta = document.getElementById(id);
  try { ta.value = JSON.stringify(JSON.parse(ta.value), null, 2); } catch {}
}

// ── Rule modal open/close/save ────────────────────────────────────────────────
function openRuleModal(rule) {
  _editingRuleId = rule ? (rule.id || null) : null;
  document.getElementById('rule-modal-title').textContent  = _editingRuleId ? 'Editar regla' : 'Nueva regla';
  document.getElementById('rule-modal-status').textContent = '';

  const r = rule || { name: '', enabled: true, condition: {}, action: { type: 'block' } };
  const c = r.condition || {};
  const a = r.action    || {};

  document.getElementById('rule-name').value         = r.name || '';
  document.getElementById('rule-url-pattern').value  = c.url_pattern  || '';
  document.getElementById('rule-host').value         = c.host         || '';
  document.getElementById('rule-method').value       = c.method       || '';
  document.getElementById('rule-header-name').value  = c.header_name  || '';
  document.getElementById('rule-header-value').value = c.header_value || '';
  document.getElementById('rule-action-type').value  = a.type || 'block';

  // Mock
  document.getElementById('rule-mock-status').value = a.type === 'mock' ? (a.status_code || '') : '';
  setCT('mock', a.type === 'mock' ? (a.content_type || '') : '');
  setHeaderRows('rule-mock-hdr-rows', a.type === 'mock' ? a.headers : null);
  document.getElementById('rule-mock-body').value   = a.type === 'mock' ? (a.body || '') : '';

  // Modify headers
  document.getElementById('rule-mod-status-cond').value = a.type === 'modify_headers' ? (c.status_code || '') : '';
  setHeaderRows('rule-mod-hdr-rows', a.type === 'modify_headers' ? a.headers : null);

  // Replace body
  document.getElementById('rule-replace-status-cond').value = a.type === 'replace_body' ? (c.status_code || '') : '';
  setCT('replace', a.type === 'replace_body' ? (a.content_type || '') : '');
  document.getElementById('rule-replace-body').value        = a.type === 'replace_body' ? (a.body || '') : '';

  updateRuleActionFields();
  document.getElementById('rule-modal-overlay').classList.add('open');
}

function updateRuleActionFields() {
  const type = document.getElementById('rule-action-type').value;
  document.getElementById('rule-fields-mock').style.display           = type === 'mock'           ? '' : 'none';
  document.getElementById('rule-fields-modify-headers').style.display = type === 'modify_headers' ? '' : 'none';
  document.getElementById('rule-fields-replace-body').style.display   = type === 'replace_body'   ? '' : 'none';
}

function closeRuleModal() {
  document.getElementById('rule-modal-overlay').classList.remove('open');
}

document.getElementById('rule-action-type').addEventListener('change', updateRuleActionFields);
document.getElementById('rule-add-btn')     .addEventListener('click', () => openRuleModal(null));
document.getElementById('rule-modal-close') .addEventListener('click', closeRuleModal);
document.getElementById('rule-modal-cancel').addEventListener('click', closeRuleModal);
document.getElementById('rule-modal-overlay').addEventListener('click', e => {
  if (e.target === document.getElementById('rule-modal-overlay')) closeRuleModal();
});

document.getElementById('rule-modal-save').addEventListener('click', async () => {
  const type = document.getElementById('rule-action-type').value;

  const condition = {
    url_pattern:  document.getElementById('rule-url-pattern').value.trim() || undefined,
    host:         document.getElementById('rule-host').value.trim()         || undefined,
    method:       document.getElementById('rule-method').value              || undefined,
    header_name:  document.getElementById('rule-header-name').value.trim() || undefined,
    header_value: document.getElementById('rule-header-value').value.trim()|| undefined,
  };

  const action = { type };

  if (type === 'mock') {
    const sc = parseInt(document.getElementById('rule-mock-status').value, 10);
    if (sc) action.status_code = sc;
    const ct = getCT('mock');
    if (ct) action.content_type = ct;
    action.headers = readHeaderRows('rule-mock-hdr-rows');
    action.body    = document.getElementById('rule-mock-body').value;
  } else if (type === 'modify_headers') {
    action.headers = readHeaderRows('rule-mod-hdr-rows');
    const sc = parseInt(document.getElementById('rule-mod-status-cond').value, 10);
    if (sc) condition.status_code = sc;
  } else if (type === 'replace_body') {
    const sc = parseInt(document.getElementById('rule-replace-status-cond').value, 10);
    if (sc) condition.status_code = sc;
    const ct = getCT('replace');
    if (ct) action.content_type = ct;
    action.body = document.getElementById('rule-replace-body').value;
  }

  const rule = {
    id:        _editingRuleId || '',
    name:      document.getElementById('rule-name').value.trim() || 'Regla',
    enabled:   true,
    condition: cleanObj(condition),
    action:    cleanObj(action),
  };

  const statusEl = document.getElementById('rule-modal-status');
  statusEl.textContent = 'Guardando…';
  statusEl.style.color = 'var(--sub)';

  try {
    const isNew = !_editingRuleId;
    const res = await fetch(isNew ? '/api/rules' : `/api/rules/${_editingRuleId}`, {
      method:  isNew ? 'POST' : 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body:    JSON.stringify(rule),
    });
    if (!res.ok) {
      const d = await res.json().catch(() => ({}));
      throw new Error(d.error || res.statusText);
    }
    closeRuleModal();
    loadRules();
  } catch (e) {
    statusEl.textContent = 'Error: ' + e.message;
    statusEl.style.color = 'var(--s5)';
  }
});

function cleanObj(o) {
  const out = {};
  for (const [k, v] of Object.entries(o)) {
    if (v !== undefined && v !== null && v !== '') out[k] = v;
  }
  return out;
}

// ── Rule from flow (context menu + detail button) ─────────────────────────────
let _ctxFlowId = null;

function escapeRegex(s) {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

async function openRuleFromFlow(flowId, preset) {
  const summary = flows.get(flowId);
  if (!summary) return;

  const path   = summary.path   || '/';
  const host   = summary.host   || '';
  const method = summary.method || '';

  let detail = null;
  if (preset === 'mock') {
    try {
      const res = await fetch(`/api/flows/${flowId}`);
      detail    = await res.json();
    } catch {}
  }

  const isBlock  = preset === 'block';
  const isMock   = preset === 'mock';

  const rule = {
    id: '',
    name: isBlock ? `Bloquear ${method} ${path}`
        : isMock  ? `Mock ${method} ${path}`
        : '',
    enabled: true,
    condition: {
      method,
      host,
      url_pattern: escapeRegex(path),
    },
    action: {
      type:         isBlock ? 'block' : isMock ? 'mock' : 'block',
      status_code:  isMock && detail ? (detail.status || 200) : undefined,
      content_type: isMock && detail ? (detail.content_type || 'application/json') : undefined,
      body:         isMock && detail ? (detail.resp_body || '') : undefined,
    },
  };

  openRuleModal(rule);
}

// Context menu — right-click on flow list items
const ctxMenu = document.getElementById('ctx-menu');

document.getElementById('flow-list').addEventListener('contextmenu', e => {
  const item = e.target.closest('.flow-item');
  if (!item) return;
  e.preventDefault();
  _ctxFlowId = item.id.replace('fi-', '');

  const x = Math.min(e.clientX, window.innerWidth  - 230);
  const y = Math.min(e.clientY, window.innerHeight - 130);
  ctxMenu.style.left    = x + 'px';
  ctxMenu.style.top     = y + 'px';
  ctxMenu.style.display = 'block';
});

function closeCtxMenu() { ctxMenu.style.display = 'none'; }

document.addEventListener('click',       closeCtxMenu);
document.addEventListener('keydown', e => { if (e.key === 'Escape') closeCtxMenu(); });

document.getElementById('ctx-mock')  .addEventListener('click', () => openRuleFromFlow(_ctxFlowId, 'mock'));
document.getElementById('ctx-block') .addEventListener('click', () => openRuleFromFlow(_ctxFlowId, 'block'));
document.getElementById('ctx-custom').addEventListener('click', () => openRuleFromFlow(_ctxFlowId, 'custom'));

// Detail panel "⚡ Regla" button
document.getElementById('rule-from-detail-btn').addEventListener('click', () => {
  if (selectedId) openRuleFromFlow(selectedId, 'custom');
});

// ── Boot ──────────────────────────────────────────────────────────────────────
setupExportModal();
connectWS();
loadJVMs();
loadFlows();
setInterval(loadJVMs, 5000);
