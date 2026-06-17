# Plan: Proxy Basic Auth for SigNoz Backend

## Status
In Progress

## Context
When self-hosted SigNoz sits behind a reverse proxy that enforces HTTP Basic Auth, the
MCP server's outbound requests are rejected by the proxy before reaching SigNoz. This is
already achievable via `SIGNOZ_CUSTOM_HEADERS=Authorization:Basic <base64>`, but operators
must hand-encode the credential. This feature adds ergonomic, dedicated env vars; the
server performs the base64 encoding. The outbound Basic credential targets the fronting
**proxy** — SigNoz's own API auth (API key / JWT) is unchanged.

## Decisions
| Topic | Decision |
|-------|----------|
| Leg | Outbound only — to a reverse proxy fronting SigNoz. |
| Header | `Authorization: Basic <base64(user:pass)>`, fixed (not configurable). |
| Scope | Configured backend only — attached only when the outbound URL equals `SIGNOZ_URL`, mirroring `customHeaders` gating in `GetClient`. Covers stdio + single-tenant http; excludes per-request multi-tenant backends. |
| Collision | When SigNoz auth uses `Authorization` (JWT-bearer path), skip Basic + warn once (redacted). Documented as unsupported. |
| Config | `SIGNOZ_BASIC_AUTH_USERNAME` + `SIGNOZ_BASIC_AUTH_PASSWORD`; both-or-neither (error if only one); username may not contain `:`. |
| Applies to | `doRequest` and `doValidationRequest` (credential validation + analytics `/me`). NOT the docs fetcher (it hits signoz.io, not the backend). |

## Approach

### 1. Config (`internal/config/config.go`)
- Add env constants `SignozBasicAuthUsername = "SIGNOZ_BASIC_AUTH_USERNAME"` and
  `SignozBasicAuthPassword = "SIGNOZ_BASIC_AUTH_PASSWORD"`.
- Add `BasicAuthHeader string` to `Config` — precomputed as `"Basic " + base64(user + ":" + pass)`,
  empty when neither var is set.
- In `LoadConfig`, read both vars (do not log them).
- In `ValidateConfig`, enforce:
  - both-or-neither: error if exactly one of username/password is set;
  - username must not contain `:`.

### 2. Client (`internal/client/client.go`)
- Add `basicAuthHeader string` field and `basicAuthWarnOnce sync.Once` to `SigNoz`.
- Extend `NewClient(...)` with a `basicAuthHeader string` parameter.
- In both `doRequest` and `doValidationRequest`, after setting the SigNoz auth header and
  before the custom-header loop:
  - if `basicAuthHeader != ""`:
    - if `strings.EqualFold(s.authHeaderName, "Authorization")` → skip; fire
      `basicAuthWarnOnce` with a redacted warning (no value);
    - else → `req.Header.Set("Authorization", s.basicAuthHeader)`.
- In the custom-header loop, also treat `Authorization` as reserved when `basicAuthHeader != ""`
  so a user custom header cannot clobber the managed Basic credential.

### 3. Handler (`internal/handler/tools/handler.go`)
- Store `basicAuthHeader` (from `cfg.BasicAuthHeader`) on `Handler`.
- In `GetClient`, pass it into `NewClient` **only when** `strings.EqualFold(signozURL, h.configURL)`
  — the same gate already used for `customHeaders`. Otherwise pass `""`.

### 4. Docs & metadata
- `README.md` — add `SIGNOZ_BASIC_AUTH_USERNAME` / `SIGNOZ_BASIC_AUTH_PASSWORD` rows to the
  config table; note: applies only to the configured `SIGNOZ_URL`; incompatible with
  JWT-bearer SigNoz auth (Basic is skipped with a warning in that case).
- `manifest.json` — add the two env vars if it enumerates server configuration.
- `plans/basic-auth.{context,plan}.md` — committed with the feature PR; flip Status to
  `In Progress` / `Done` as work proceeds.

## Files to Modify
- `internal/config/config.go` — env constants, `BasicAuthHeader`, parse + validate.
- `internal/config/config_test.go` — both-or-neither, colon rejection, base64 correctness.
- `internal/client/client.go` — field, `NewClient` param, injection + collision in
  `doRequest` and `doValidationRequest`, reserved `Authorization`.
- `internal/client/client_test.go` — header present when configured; skipped + warns on
  `Authorization` mode; user custom `Authorization` cannot override.
- `internal/handler/tools/handler.go` — thread + URL-gated injection.
- `internal/handler/tools/*_test.go` — gating (attached for configured URL, not for other tenant).
- `README.md`, `manifest.json` — config docs/metadata.

## Verification
- **Unit**: config both-or-neither + colon rejection + base64; client header present/absent/
  collision-warn; reserved `Authorization`.
- **Integration**: handler gating by URL match.
- **Manual/E2E** (delegate to a subagent per CLAUDE.md): local nginx with `auth_basic` fronting
  SigNoz; tool call succeeds with creds, 401s without; never print credentials.

## Out of Scope
Inbound Basic auth to the MCP server; SigNoz login→JWT exchange; multi-tenant per-request
proxy creds; `Proxy-Authorization` variant; docs-fetcher proxying.
