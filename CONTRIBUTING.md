# Contributing to Proxito

## Prerequisites

- Go 1.26+ (the right toolchain downloads automatically via `GOTOOLCHAIN=auto`)
- A JDK if you want to test the JVM attach feature

## Running locally

```bash
git clone https://github.com/geordym/proxito.git
cd proxito
go build -o proxito .    # macOS / Linux
go build -o proxito.exe .  # Windows
./proxito
```

Open http://localhost:8282. The frontend is vanilla JS — edit `frontend/` and refresh, no build step.

---

## Branch naming

| Prefix | Use for |
|---|---|
| `feat/` | New feature (`feat/rules-engine`) |
| `fix/` | Bug fix (`fix/ws-reconnect`) |
| `ci/` | Pipeline changes (`ci/add-govulncheck`) |
| `docs/` | Documentation only (`docs/contributing`) |
| `chore/` | Maintenance with no functional change (`chore/bump-go`) |

Branch names are lowercase, hyphen-separated. No ticket numbers required, but including the issue number is welcome (`feat/16-rules-engine`).

---

## Commit messages

This project uses **Conventional Commits**:

```
<type>(<scope>): <short summary>

<optional body>
```

**Types:** `feat` · `fix` · `ci` · `docs` · `chore` · `refactor` · `test`

**Rules:**
- Summary in imperative mood, lowercase, no period: `add rule engine`, not `Added rule engine.`
- Body explains *why*, not *what* — the diff already shows what changed
- Reference issues with `closes #N` or `fixes #N` in the body or footer

**Examples:**

```
feat: add automated rules engine (closes #6)
fix: format Go files, patch 3 CVEs
ci: add GitHub Actions pipeline (closes #14)
docs: expand CONTRIBUTING, add SECURITY.md (closes #16)
```

---

## Dependency policy

**New direct dependencies are frozen.** The CI `dependency-freeze` job will reject any PR that adds a new package to the non-indirect section of `go.mod`.

**Why:** Proxito ships as a single self-contained binary. Every new dependency increases attack surface, binary size, and long-term maintenance cost.

**Current approved direct dependencies:**

| Package | Version | Role |
|---|---|---|
| `github.com/lqqyt2423/go-mitmproxy` | v1.9.5 | MITM proxy engine |
| `github.com/gorilla/websocket` | v1.5.3 | WebSocket hub |

**Indirect dependencies** (transitive, marked `// indirect` in `go.mod`) may be bumped for security patches without approval.

**To request a new dependency:** open an issue explaining what it does, why the stdlib or existing deps can't cover it, and what the binary size impact is. It will be discussed before any PR is opened.

---

## Testing

There are currently no automated unit tests. The CI runs `go test ./...` and passes on an empty suite.

If you add a feature, adding a test for the core logic is welcome but not required for a PR to be merged. When adding tests:

- Table-driven tests (`t.Run`) preferred
- No mocking of internal packages — test real behavior
- Tests that require a running proxy or browser are out of scope

---

## Code style

- Run `gofmt -w .` before committing — the CI format check will fail otherwise
- `go vet ./...` must pass
- Frontend: vanilla JS + CSS, no bundler, no framework

---

## Release process

Releases are created by pushing a `vMAJOR.MINOR.PATCH` tag to `main`. The release workflow builds binaries for all platforms and publishes a GitHub Release automatically.

**Versioning:**

| Change | Version bump |
|---|---|
| New user-visible feature | `MINOR` (e.g. `v0.2.0`) |
| Bug fix or security patch | `PATCH` (e.g. `v0.1.1`) |
| Breaking change to config/API | `MAJOR` (e.g. `v1.0.0`) |

Only the maintainer creates tags. If you think a release is due, open an issue or comment on a PR.

---

## Reporting bugs

Use the [Bug report](.github/ISSUE_TEMPLATE/bug_report.md) template. Include OS, architecture, Proxito version, and steps to reproduce.

## Reporting security vulnerabilities

See [SECURITY.md](SECURITY.md).
