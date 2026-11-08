# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - Unreleased

Initial release.

### Added

- Typed Go client for the Cotizave API v2, with zero third-party dependencies.
- Resources: `Rates` (list, get, snapshot, global), `History` (get, daily,
  stats, raw, iterate, limits), `Convert` / `ConvertBulk`, FX tools (`Markets`,
  `Spread`, `Route`, `BankRates`), catalogs (`Countries`, `Currencies`, `Banks`,
  `Issuers`), `Holidays` (list, check, next, business days) and `Crypto`.
- Low-level `Request` (with conditional GETs) and `Export` (CSV/XML).
- Context-first API: every call takes a `context.Context`.
- Security hardening: key sent only as the `X-API-Key` header and never logged
  or exposed via `Config()` / `String()`; origin pinning; redirect refusal;
  HTTPS enforcement; response-size cap; header-injection rejection.
- Retry policy for idempotent requests: connection failures, `5xx` and burst
  `429` (honoring `Retry-After`) with exponential backoff, full jitter and a
  total time budget; monthly-quota `429` is never retried.
- Typed error hierarchy (`*Error`, `*RateLimitError`) with ergonomic predicates
  (`IsNotFound`, `IsRateLimited`, `IsTimeout`, …).
