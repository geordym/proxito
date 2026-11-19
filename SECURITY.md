# Security Policy

## Supported versions

| Version | Supported |
|---|---|
| `0.x` (latest tag) | ✅ |
| older tags | ❌ — upgrade to latest |

## Reporting a vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Send a report to **ggeordymm@gmail.com** with:

1. A short description of the vulnerability
2. Steps to reproduce (or a proof-of-concept)
3. The version affected (output of `./proxito --version` or the git tag)
4. Your assessment of severity / exploitability

You will receive an acknowledgement within **48 hours** and a status update within **7 days**.

## Disclosure policy

- Vulnerabilities are fixed in a patch release (`PATCH` version bump) as soon as possible.
- Once a fix is released, the issue is disclosed publicly in the GitHub release notes.
- Credit is given to the reporter unless anonymity is requested.

## Scope

Proxito is a **local developer tool** — it binds by default to `127.0.0.1`. The threat model is a local attacker or a compromised dependency, not a remote attacker.

In scope:
- Arbitrary code execution via crafted proxy traffic
- CA key or settings file exposure
- Path traversal in the embedded file server
- Dependency vulnerabilities with a reachable exploit path (flag these so we can bump the dep)

Out of scope:
- Attacks that require physical access to the machine
- Denial-of-service on `localhost` (no SLA for a dev tool)
- Self-XSS in the local UI (you are the only user)

## Dependency vulnerabilities

The CI pipeline runs `govulncheck` on every push. If you find a CVE in a transitive dependency before CI does, report it via the channel above and we will bump the indirect dep in a patch release.
