# Feature: Proxy Basic Auth for SigNoz Backend — Context & Discussion

## Original Prompt
> add a new authentication feature with signoz backend using basic auth

## Reference Links
- RFC 7617 — The 'Basic' HTTP Authentication Scheme
- Existing `SIGNOZ_CUSTOM_HEADERS` mechanism (README.md config table) — documented for "when SigNoz is behind a reverse proxy requiring auth"

## Key Decisions & Discussion Log

### 2026-06-17 — Which leg of auth?
- Considered three interpretations: (A) outbound to a proxy fronting SigNoz, (B) outbound to SigNoz itself via a login→JWT exchange, (C) inbound Basic auth to the MCP server.
- **Resolved: (A) outbound to a reverse proxy fronting SigNoz.** SigNoz's own API auth (API key / JWT) is untouched; Basic only satisfies the fronting proxy.

### 2026-06-17 — Why a dedicated feature when SIGNOZ_CUSTOM_HEADERS already works?
- `client.go` `doRequest` already forwards a custom `Authorization: Basic <base64>` header (it is not reserved when the SigNoz key uses the `SIGNOZ-API-KEY` header). So proxy Basic is already possible by hand-encoding.
- **Resolved: the sole justification is ergonomics** — operators supply plain `username`/`password` and the server does the base64 encoding. (Secret-hygiene and collision-correctness were offered but not chosen as motivations; collision is still handled as a correctness matter below.)

### 2026-06-17 — Deployment scope
- stdio hardcodes the outbound auth header to `SIGNOZ-API-KEY` (`server.go:1109`) → `Authorization` always free.
- http forwards a client JWT as `Authorization: Bearer` (`server.go:1288`) → only path where outbound `Authorization` is already used.
- Multi-tenant: custom headers attach only when the tenant URL equals the configured `SIGNOZ_URL` (`handler.go:90-93`).
- **Resolved: configured backend only** (stdio + single-tenant http), mirroring the `customHeaders` gating. Excludes per-request multi-tenant backends; naturally avoids the JWT collision in the common case.

### 2026-06-17 — Authorization-header collision policy
- A reverse proxy expects `Authorization: Basic`; SigNoz's JWT path also uses `Authorization: Bearer`. They cannot share one header.
- Existing reserved-header logic in `doRequest` (`client.go:362-368`) already logs + skips colliding custom headers — but it logs the header **value**, which would leak the credential.
- **Resolved: when outbound SigNoz auth uses `Authorization` (JWT path), skip the Basic header and emit a one-time, redacted warning.** Documented as an unsupported combination. The managed credential is kept out of the value-logging path.

### 2026-06-17 — Config API + ADR
- **Resolved: `SIGNOZ_BASIC_AUTH_USERNAME` + `SIGNOZ_BASIC_AUTH_PASSWORD`**, both-or-neither (config-load error if only one set), username may not contain `:` (RFC 7617).
- **Resolved: no ADR.** The plan + this context file + README capture the rationale; the change is small and the public surface (two env vars) is low-risk.

## Open Questions
- (none — all resolved during grilling)
