# Contributing to Proxito

## Prerequisites

- Go 1.26+ (or any version with `GOTOOLCHAIN=auto` — the right toolchain downloads automatically)
- A JDK if you want to test the JVM attach feature

## Running locally

```bash
git clone https://github.com/geordym/proxito.git
cd proxito
go build -o proxito.exe .   # Windows
go build -o proxito .       # macOS / Linux
./proxito
```

Open http://localhost:8282.

## Project structure

```
proxito/
├── main.go              # entry: flags, JDK detection, proxy + UI startup
├── proxy/
│   ├── flow.go          # Flow struct, Store, InterceptHub
│   └── proxy.go         # go-mitmproxy Addon (Request/Response hooks)
├── api/
│   ├── server.go        # HTTP mux + Server struct
│   ├── handlers.go      # REST handlers
│   ├── settings.go      # GET/POST /api/settings
│   └── ws.go            # WebSocket Hub
├── jvm/
│   ├── manager.go       # JVM attach logic
│   └── detect.go        # JDK detection
├── config/
│   └── config.go        # ~/.proxito/settings.json
└── frontend/
    ├── index.html
    └── app.js
```

The frontend is vanilla JS/CSS — no build step, no bundler. Edit and refresh.

## Making changes

1. Fork the repo and create a branch: `git checkout -b fix/my-fix`
2. Make your changes
3. Test manually: build and run, then browse to http://localhost:8282
4. Open a Pull Request against `main`

## Reporting bugs

Use the [Bug report](.github/ISSUE_TEMPLATE/bug_report.md) issue template. Include:
- OS and architecture
- Proxito version (from the release filename or `git log -1`)
- Steps to reproduce
- What you expected vs what happened

## Code style

- Standard Go formatting (`gofmt` / `goimports`)
- No external frontend frameworks — keep it vanilla JS
- Keep the binary self-contained: no runtime deps, everything embedded
