# Security Policy

## Reporting a vulnerability

Please report security issues privately. **Do not open a public GitHub issue for
a vulnerability.**

- Email: **security@cotizave.com**
- Or use GitHub's [private vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability)
  on this repository.

Include a description, reproduction steps, affected versions, and impact. We aim
to acknowledge within a few business days and will coordinate a fix and
disclosure timeline with you.

## Handling API keys

This SDK is designed for **server-side** use.

- Never embed a private API key in a public binary, a mobile app, or any
  client-distributed program.
- Load the key from a server-side secret (environment variable or secret
  manager), never from source control.

The SDK sends your key only as the `X-API-Key` header to the configured base
URL, never in a URL or query string. It does not log the key, persist it, place
it in error messages or in the `Config()` / `String()` output, send telemetry,
or make requests to any other origin. It pins every request to the configured
origin and refuses to follow redirects, so the key cannot be forwarded off-host.

## Supply chain

- The SDK has **zero third-party dependencies** — it is built entirely on the Go
  standard library. Installing it adds nothing else to your module graph.
- `govulncheck` runs in CI on every change and must report no vulnerabilities.
- Releases are built and gated on GitHub-hosted runners.

## Supported versions

The latest released minor version receives security fixes. Pin a version with
Go modules (`go get github.com/cotizave/cotizave-go@vX.Y.Z`).
