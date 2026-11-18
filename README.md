# Proxito

Personal HTTP/S MITM proxy for inspecting and intercepting traffic — focused on JVM applications. Inspired by [HTTP Toolkit](https://httptoolkit.com).

Single binary: no dependencies, no install. Download, run, done.

## Features

- **HTTPS interception** via auto-generated CA certificate (powered by [go-mitmproxy](https://github.com/lqqyt2423/go-mitmproxy))
- **JVM agent injection** — attach to any running Java process without restarting it
- **Real-time flow inspection** — view request/response headers and bodies as they happen
- **Intercept & block** — pause requests in-flight, then resume or block them
- **Export requests** in multiple formats: cURL, fetch, axios, Python requests, raw HTTP
- **Embedded web UI** — dark navy theme, no external dependencies
- **Auto JDK detection** from `~/.jdks/` (IntelliJ) and system paths

## Download

Grab the latest binary for your platform from [Releases](https://github.com/geordym/proxito/releases):

| Platform | File |
|---|---|
| Windows x64 | `proxito-windows-amd64.exe` |
| macOS Apple Silicon | `proxito-darwin-arm64` |
| macOS Intel | `proxito-darwin-amd64` |
| Linux x64 | `proxito-linux-amd64` |
| Linux ARM64 | `proxito-linux-arm64` |

## Quick start

```bash
# Run with defaults (proxy: 8080, UI: 8282)
./proxito

# Custom ports
./proxito --port 9090 --ui-port 9191
```

Then open **http://localhost:8282** in your browser.

### Configure your proxy

Point your browser or system proxy to `127.0.0.1:8080`.

### Trust the CA certificate

To intercept HTTPS you need to trust Proxito's CA cert:

1. Open **http://localhost:8282** → go to **Ajustes**
2. Click **Descargar certificado CA**
3. Install it in your OS/browser as a trusted root CA

> The cert is stored at `~/.proxito/mitmproxy-ca-cert.pem`

### Intercept JVM traffic

1. Go to the **Interceptar** tab
2. Find your Java process and click **Adjuntar**
3. Traffic from that JVM will appear in the **Ver** tab

## Building from source

**Requirements:** Go 1.26+ (automatically downloaded via `GOTOOLCHAIN` if needed)

```bash
git clone https://github.com/geordym/proxito.git
cd proxito
go build -o proxito .
./proxito
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--host` | `127.0.0.1` | Proxy listen address |
| `--port` | `8080` | Proxy port |
| `--ui-port` | `8282` | Web UI port |

## Runtime files

Proxito stores its data in `~/.proxito/`:

| File | Description |
|---|---|
| `mitmproxy-ca-cert.pem` | CA certificate to trust in your browser/OS |
| `http-proxy-agent.jar` | Java agent (extracted on first run) |
| `settings.json` | Saved settings (java path, ports) |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT — see [LICENSE](LICENSE).

The embedded Java agent (`http-proxy-agent-1.3.9-dist.jar`) is from [HTTP Toolkit](https://httptoolkit.com) and is licensed under Apache 2.0 — see [NOTICES](NOTICES).
