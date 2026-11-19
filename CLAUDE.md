# Proxito — CLAUDE.md

Personal HTTP Toolkit Pro alternative. Single Go binary: MITM proxy + embedded web UI + JVM agent injection.

## For AI agents — read these first

Before making any change, read:

- **[CONTRIBUTING.md](CONTRIBUTING.md)** — branch naming, Conventional Commits, dependency freeze policy, testing rules, release process.
- **[SECURITY.md](SECURITY.md)** — threat model, what is in/out of scope, how to handle dependency CVEs.

Key rules that affect every PR:
- No new **direct** dependencies without an approved issue (the `dependency-freeze` CI job enforces this).
- Run `gofmt -w .` and `go vet ./...` before committing — CI rejects unformatted code.
- Commit messages follow Conventional Commits: `feat:`, `fix:`, `ci:`, `docs:`, `chore:`, `refactor:`, `test:`.

## Build & run

```
go build -C D:\herramientas\httpproxy\proxito -o proxito.exe .
.\proxito.exe [--host 127.0.0.1] [--port 8080] [--ui-port 8282]
```

Go 1.26+ required (go-mitmproxy v1.9.5 constraint). The toolchain auto-downloads via GOTOOLCHAIN.

## Project layout

```
proxito/
├── main.go              # entry: flags, JDK detection, proxy + UI startup
├── proxy/
│   ├── flow.go          # Flow struct, Store (thread-safe, max 1000), InterceptHub
│   ├── proxy.go         # go-mitmproxy Addon (Request/Response hooks)
│   └── rules.go         # RuleEngine: block/mock/modify_headers/replace_body, persists to rules.json
├── api/
│   ├── server.go        # HTTP mux, routes, Server struct
│   ├── handlers.go      # REST handlers: JVMs, flows, intercept, cert download
│   ├── rules.go         # REST handlers: GET/POST/PUT/DELETE /api/rules
│   ├── settings.go      # GET/POST /api/settings + refreshJDKs
│   └── ws.go            # WebSocket Hub (gorilla/websocket)
├── jvm/
│   ├── manager.go       # List JVMs, Attach via java agent, tools.jar detection
│   └── detect.go        # DetectJDKs, BestJava — scans ~/.jdks/ + OS paths
├── config/
│   └── config.go        # Config{JavaBin,ProxyHost,ProxyPort}, Load/Save ~/.proxito/settings.json
├── frontend/
│   ├── index.html       # Four-view SPA: Interceptar + Ver + Reglas + Ajustes (HTTP Toolkit dark navy)
│   └── app.js           # Vanilla JS: WebSocket, flow list, export modal, rules UI, settings
├── http-proxy-agent-1.3.9-dist.jar  # embedded, extracted to ~/.proxito/ at startup
├── CONTRIBUTING.md      # branch naming, commits, dependency policy, testing, release process
├── SECURITY.md          # vulnerability reporting, scope, disclosure policy
├── NOTICES              # License attributions
└── go.mod               # module proxito, go 1.26.0
```

## Key dependencies

| Package | Role |
|---|---|
| `github.com/lqqyt2423/go-mitmproxy v1.9.5` | MITM proxy engine, CA cert management |
| `github.com/gorilla/websocket v1.5.3` | WebSocket hub for real-time flow broadcast |
| `http-proxy-agent-1.3.9-dist.jar` | HTTP Toolkit Java agent (Apache 2.0) — injects proxy into running JVM |

## Runtime data (`~/.proxito/`)

| File | Created by |
|---|---|
| `mitmproxy-ca.pem` | go-mitmproxy — combined CA cert+key (internal) |
| `mitmproxy-ca-cert.pem` | go-mitmproxy — cert only, served at `GET /api/cert` |
| `http-proxy-agent.jar` | extracted from embedded binary on first run (SHA-256 checked) |
| `settings.json` | `config.Save()` — persists JavaBin, ProxyHost, ProxyPort |

The user must trust `mitmproxy-ca-cert.pem` in their OS/browser to intercept HTTPS.

## REST API

```
GET    /api/jvms                  list running JVMs
POST   /api/jvms/{pid}/attach     attach proxy agent to JVM
GET    /api/flows                 list all flows (summary)
GET    /api/flows/{id}            full flow detail (headers + body)
POST   /api/flows/{id}/resume     resume paused flow
POST   /api/flows/{id}/block      block paused flow (returns 403)
POST   /api/intercept             {"enabled": bool}
GET    /api/cert                  download CA cert as PEM
GET    /api/settings              get current config + detected JDKs
POST   /api/settings              save config (java_bin, proxy_host, proxy_port)
GET    /api/rules                 list all rules
POST   /api/rules                 create rule
PUT    /api/rules/{id}            update rule
DELETE /api/rules/{id}            delete rule
GET    /ws                        WebSocket — real-time flow events
```

## WebSocket message types

| `type` | Direction | Key fields |
|---|---|---|
| `flow_request` | server→client | `id, method, url, host, path, state, started_at` |
| `flow_response` | server→client | `id, method, url, host, path, status, size, started_at, done_at, state` |
| `flow_paused` | server→client | `id, state:"paused"` |
| `flow_resumed` | server→client | `id, state:"active"` |
| `flow_done` | server→client | `id, state:"blocked"\|"error"` |
| `intercept_changed` | server→client | `enabled: bool` |
| `settings_changed` | server→client | `java_bin` |
| `jvm_attached` | server→client | `pid` |

`flow_response` and state messages are **merged** (not replaced) into the existing flow entry via `Object.assign` so `started_at` from `flow_request` is preserved for duration calculation.

## go-mitmproxy Addon pattern

```go
// proxy/proxy.go
type Addon struct {
    gmp.BaseAddon        // no-op implementations for unused hooks
    store     *Store
    intercept *InterceptHub
    broadcast func(any)
}

func (a *Addon) Request(f *gmp.Flow)  { /* create Flow, optionally pause */ }
func (a *Addon) Response(f *gmp.Flow) { /* update Flow, broadcast */ }
```

To block a flow in `Request`: set `f.Response = &gmp.Response{StatusCode: 403, ...}` — go-mitmproxy returns this to the client without hitting upstream.

## JDK detection & Java attach

`DetectJDKs()` scans `~/.jdks/` (IntelliJ-managed) and OS paths. `BestJava()` prefers Java 11–21 from `~/.jdks/`.

Java 8 needs `tools.jar` on the classpath for the attach API; Java 9+ does not. `jvm.Manager.toolsJarFlags()` handles this automatically via three detection strategies (LookPath→relative, `$JAVA_HOME`, `-XshowSettings:property`).

The saved `java_bin` path in settings is what gets passed to the agent. `SetJavaBin()` invalidates the tools.jar cache so the new Java's tools.jar is re-detected.

## Rules engine (`proxy/rules.go`)

Four action types: `block`, `mock`, `modify_headers`, `replace_body`.

**Request phase** (`ApplyRequest`) — fires before upstream is hit:
- `block` → sets `f.Response` to 403, returns immediately (upstream never called).
- `mock` → sets `f.Response` to configured status/body/headers, returns immediately.

**Response phase** (`ApplyResponse`) — fires after upstream responds:
- `modify_headers` → adds/overwrites headers on the real response.
- `replace_body` → replaces body bytes (and optionally Content-Type) on the real response.

Conditions: `url_pattern` (regexp), `host`, `method`, `header_name`+`header_value`, `status_code` (response-phase only).

Rules persist to `~/.proxito/rules.json`. Thread-safe via `sync.RWMutex`.

## Frontend

**Stack:** Vanilla JS + CSS, no bundler, no framework. All CSS variables in `:root` on `index.html`.

**Theme:** Dark navy (`--bg: #1b2638`) inspired by HTTP Toolkit. Method badges have per-method colors (GET green, POST blue, PUT orange, PATCH purple, DELETE red).

**Four views** — `switchView(name)` toggles `.active` on `.view` divs and `.nav-btn` tabs:

| View | ID | Description |
|---|---|---|
| Interceptar | `#view-intercept` | Grid of JVM process cards. Each card has ☕ icon, process name (extracted from classpath), PID, Adjuntar button. Auto-refreshes every 5s. |
| Ver | `#view-inspect` | Left sidebar (310px): searchable flow list. Right: full-height detail panel with Request \| Response columns. Right-click a flow → context menu to create a rule pre-filled from that flow. |
| Reglas | `#view-rules` | List of saved rules with enable/disable toggle, edit, delete. Rule modal covers all four action types. |
| Ajustes | `#view-settings` | JDK picker + proxy host/port. Auto-detection banner when Java was auto-selected. |

**Flow list items** (`#flow-list`): built by `buildFlowItem(id, f)`, prepended on new flows. Each item shows method badge + path + host + status badge. `.active` = selected, `.paused` = intercepted. Filter via `matchesFilter()` / `refilter()`.

**Detail panel**: `renderDetail(id)` fetches `/api/flows/{id}`, populates request/response columns via `headersBlock()` + `bodyBlock()`. Shows paused banner with Resume/Block buttons when `state === 'paused'`. Exposes `_copyData` for the export modal.

**Export modal** (`#export-overlay`): bottom-sheet that slides up. Opened by the `</> Exportar` button in the detail URL bar (only visible when a flow is selected). Five format tabs — switching tabs calls `renderExportCode(fmt)` which runs the formatter and sets `<pre>` text content. Copy button writes to clipboard and shows `✓ Copiado`. Closes on ✕, outside click, or Escape.

**Export formatters** (all in `app.js`):

| Function | Output |
|---|---|
| `fmtCurl(d)` | `curl -X METHOD 'url' -H '...' --data-raw '...'` |
| `fmtFetch(d)` | `fetch(url, { method, headers, body })` |
| `fmtAxios(d)` | `axios({ method, url, headers, data })` |
| `fmtPython(d)` | `requests.METHOD(url, headers={}, json={})` |
| `fmtRaw(d)` | RFC HTTP text with CRLF line endings |

All formatters strip proxy-internal headers (`proxy-connection`, `transfer-encoding`, `content-length`, `host`) via the `SKIP_HDR` set. JSON body detection is via `Content-Type: application/json`.
