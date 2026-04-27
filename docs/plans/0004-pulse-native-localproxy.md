# Pulse-Native Local HTTPS Proxy

This ExecPlan is a living document. Update Progress, Surprises & Discoveries, Decision Log, and Outcomes & Retrospective as work proceeds.

## Purpose / Big Picture

Pulse currently uses embedded Caddy modules in `internal/localproxy` to serve local HTTPS reverse-proxy routes for `pulse dev` and standalone `pulse run`. The goal is to replace that embedded Caddy dependency with a small Pulse-native Go implementation that preserves the current public `localproxy` API and user-visible behavior.

When this plan is complete, `pulse dev` and standalone runtime development mode still expose HTTPS URLs such as `https://api.<workspace>.localhost`, `https://console.<workspace>.localhost`, `https://mcp.<workspace>.localhost`, and `https://pulse.<workspace>.localhost`, but the implementation uses only the Go standard library for routing, TLS certificates, trust installation, reverse proxying, and lifecycle management. Caddy imports and Caddy-only dependencies are removed from the repository.

## Progress

- [x] (2026-04-27 17:06Z) Created this ExecPlan and assigned historical ID 0004.
- [ ] Replace Caddy-backed route/config generation with a Pulse-native route table.
- [ ] Implement Pulse local CA, leaf certificate generation, and SAN validation.
- [ ] Implement injectable OS trust installation without Caddy or global process mutation.
- [ ] Implement HTTPS reverse proxy serving and optional HTTP-to-HTTPS redirect serving.
- [ ] Update call sites and tests while preserving public `internal/localproxy` API names and URL behavior.
- [ ] Remove `internal/localproxy/caddyimports.go` and Caddy dependencies, then run full validation.

## Surprises & Discoveries

- Current `internal/localproxy/proxy_test.go` names local proxy and trust defaults as opt-in: `Enabled()` currently defaults to `false`, and `SkipInstallTrust()` currently defaults to `true`. The requested target behavior says `PULSE_LOCAL_PROXY` should default to enabled and `PULSE_LOCAL_PROXY_SKIP_TRUST_INSTALL` should default to false. Implementation must decide whether to update behavior to the requested target and adjust tests, or preserve actual current behavior if callers rely on it. Record the final decision in the Decision Log.
- Current `runtimeapp/app.go` only disables standalone proxy startup when `PULSE_LOCAL_PROXY == "0"`. The requested behavior allows replacing this with `localproxy.Enabled()` only if tests cover `"0"`, `"false"`, `"no"`, and `"off"`.
- Current Caddy setup passes both `HTTPPort` and `HTTPSPort` into the Caddy HTTP app but configures only the `pulse` server with `Listen: :<HTTPSPort>`. The replacement should implement an HTTP redirect listener unless implementation investigation proves Caddy did not bind HTTP in this configuration.

## Decision Log

- Decision: Keep the replacement inside `internal/localproxy` and preserve all exported names from that package.
  Rationale: `cmd/pulse/dev_supervisor.go` and `runtimeapp/app.go` already depend on `localproxy.Config`, `Start`, `Proxy.Routes`, and URL helper functions. Preserving the API keeps the migration focused on implementation parity.
  Date/Author: 2026-04-27 / Codex

- Decision: Use Go standard library primitives for the custom proxy.
  Rationale: `net/http`, `net/http/httputil`, `crypto/x509`, `crypto/ecdsa`, `tls`, `os/exec`, and `net` are sufficient for the required local-only behavior, and the repository explicitly prefers minimal dependencies.
  Date/Author: 2026-04-27 / Codex

## Outcomes & Retrospective

Not yet completed.

## Context and Orientation

The Caddy dependency is concentrated in `internal/localproxy/proxy.go` and `internal/localproxy/caddyimports.go`. Current direct and indirect dependency entries appear in `go.mod` and `go.sum` for `github.com/caddyserver/caddy/v2`, `github.com/caddyserver/certmagic`, and `github.com/caddyserver/zerossl`.

The public package surface to preserve is:

    type Config
    type Routes
    type Proxy
    Enabled() bool
    HTTPPort() int
    HTTPSPort() int
    SkipInstallTrust() bool
    FrontendOverride() string
    DiscoverWorkspace(root, fallback string) string
    DiscoverFrontendUpstream(root string) string
    BuildConfig(cfg Config) Config
    Start(cfg Config) (*Proxy, error)
    (*Proxy).Close() error
    (*Proxy).Routes() Routes
    ConsoleAppURL(routes Routes, appID string) string
    MCPSSEURL(routes Routes, appID string) string

`Config` fields must remain `Workspace`, `APIHost`, `ConsoleHost`, `MCPHost`, `FrontendHost`, `APIUpstream`, `DashboardUpstream`, `FrontendUpstream`, `HTTPPort`, `HTTPSPort`, `SkipInstallTrust`, and `Verbose`. `Routes` fields must remain `APIHost`, `ConsoleHost`, `MCPHost`, `FrontendHost`, `APIURL`, `ConsoleURL`, `MCPBaseURL`, and `FrontendURL`.

Main call sites are:

- `cmd/pulse/dev_supervisor.go`: starts the proxy before launching the child app, sets `PULSE_LOCAL_PROXY=0` in the child, and sets `PULSE_PUBLIC_BASE_URL` when a proxy exists.
- `runtimeapp/app.go`: starts the standalone local HTTPS proxy when the runtime was not launched by the supervisor.

Existing helpers in `internal/localproxy/proxy.go` define important normalization behavior. Preserve `normalizeUpstream`, `normalizeHost`, `sanitizeLabel`, `DiscoverWorkspace`, `DiscoverFrontendUpstream`, `BuildConfig`, `routesFor`, `routeSubjects`, `hostURL`, `ConsoleAppURL`, and `MCPSSEURL` behavior unless this plan records a deliberate decision to change an environment default.

## Milestones

Milestone 1 introduces a Caddy-independent route table. This is complete when tests can inspect resolved API, console, MCP, frontend, and `/__pulse/config` routes without building Caddy JSON.

Milestone 2 implements local certificate storage. This is complete when the package can generate or reuse a Pulse development CA, generate a leaf certificate with the expected SAN DNS names, store private keys with `0600`, store directories with `0700`, and regenerate leaf certificates when missing, expired, near expiry, not covered by SANs, or signed by a changed CA.

Milestone 3 implements trust installation. This is complete when trust installation is injectable for tests and has OS-specific files for Darwin, Linux, Windows, and other platforms. Trust installation should be idempotent, should not mutate Go global process state, and should warn clearly when unsupported or unsuccessful.

Milestone 4 implements the servers. This is complete when `Start` validates without opening listeners, prepares certificates, optionally installs trust, starts HTTPS reverse proxy serving, starts HTTP redirects when appropriate, and returns a `Proxy` whose `Close` method is nil-safe, idempotent, graceful, and releases ports.

Milestone 5 removes Caddy. This is complete when `internal/localproxy` imports no `github.com/caddyserver` packages, `internal/localproxy/caddyimports.go` is deleted, `go mod tidy` removes Caddy-only modules, and `go list -m all` shows no Caddy, CertMagic, or ZeroSSL modules unless another dependency genuinely requires them.

## Plan of Work

Start by carving the route model out of the Caddy JSON model. Replace `configJSON`, `proxyRoutes`, and `proxyRoute` with small route structs such as `proxyRoute{host, path, upstream string, rewriteHost bool}` and a route table that matches host case-insensitively after stripping any port from `req.Host`. The frontend host must register the exact `/__pulse/config` route before the catch-all frontend route so API config routing wins.

Next add certificate code under `internal/localproxy/cert.go`. Use `PULSE_DEV_CACHE_DIR` when set, otherwise `os.UserCacheDir()`, and store files under a Pulse-specific directory like `<cache-root>/pulse/localproxy/`. Generate an ECDSA P-256 or RSA 2048 CA certificate, a matching CA key, a leaf certificate, and a leaf key. Keep the CA long-lived, keep the leaf shorter-lived, and expose small internal helpers so tests can load the generated CA into an `http.Client` root pool without touching the system trust store.

Add trust installers under:

    internal/localproxy/trust.go
    internal/localproxy/trust_darwin.go
    internal/localproxy/trust_linux.go
    internal/localproxy/trust_windows.go
    internal/localproxy/trust_other.go

The shared file should define the injectable function used by `Start`. Darwin should use `security` with user-level trust where possible. Windows should use `certutil -user -addstore Root <cert>`. Linux should support common `trust anchor`, `update-ca-certificates`, and `update-ca-trust` mechanisms when available, and otherwise return a clear warning that HTTPS is still serving with an untrusted local CA.

Then implement serving with `net/http` and `net/http/httputil.ReverseProxy`. Build upstream targets as `http://<normalized-upstream>`, preserve path and query, return `502` on upstream errors, return `404` for unknown host/path combinations, and keep default reverse proxy streaming behavior for SSE and WebSocket traffic. For API, console, MCP, and frontend `/__pulse/config`, preserve the incoming `Host` header. For the frontend catch-all route, set `req.Host` to the upstream host:port to match the old Caddy `Host: {http.reverse_proxy.upstream.hostport}` behavior. Set useful `X-Forwarded-For`, `X-Forwarded-Host`, and `X-Forwarded-Proto`; HTTPS requests should report `X-Forwarded-Proto: https`.

Finally update the call sites only as needed. `cmd/pulse/dev_supervisor.go` should continue to start the proxy before the child app and pass `PULSE_LOCAL_PROXY=0` plus `PULSE_PUBLIC_BASE_URL=<routes.APIURL>`. `runtimeapp/app.go` should continue to avoid a second proxy when launched by the supervisor or when local proxy is disabled. The runtime banner should still use routes for API, dashboard, MCP SSE, frontend, and DB Studio URLs.

## Concrete Steps

1. Read current `internal/localproxy/proxy.go`, `internal/localproxy/proxy_test.go`, `cmd/pulse/dev_supervisor.go`, `runtimeapp/app.go`, and `go.mod`.
2. Add a route table abstraction in `internal/localproxy/proxy.go` or a new `internal/localproxy/routes.go`; keep `routesFor` and `routeSubjects` tests intact.
3. Replace Caddy JSON tests with route table tests:

        TestRouteTableIncludesExpectedHosts
        TestCertificateSubjects
        TestStartRejectsMissingAPIUpstream
        TestStartRejectsMissingHost

4. Add certificate generation and cache helpers in `internal/localproxy/cert.go`, plus tests for SANs, CA reuse, leaf regeneration, and file permissions where practical.
5. Add injectable trust installation in `internal/localproxy/trust.go` and OS-specific implementations. Add tests for `SkipInstallTrust` using a mocked installer.
6. Implement `Proxy` fields for `routes`, `httpsServer`, optional `httpServer`, listeners, goroutine coordination, and `closeOnce`.
7. Implement `Start` in this order: normalize config, validate config, compute routes and route table, prepare certificates, optionally install trust, create HTTPS listener, create HTTP redirect listener if `HTTPPort != HTTPSPort`, start server goroutines, return `*Proxy`.
8. Implement `Close` as nil-safe and idempotent. It should gracefully shut down servers with a short timeout, close listeners, wait for goroutines, and return `errors.Join` of relevant errors.
9. Add integration tests using high random ports and `httptest` upstreams:

        TestProxyRoutesToAPI
        TestProxyRoutesDashboardHosts
        TestProxyRoutesFrontendConfigToAPI
        TestProxyRoutesFrontendCatchAllToFrontendAndRewritesHost
        TestUnknownHostReturns404
        TestHTTPRedirectKnownHost
        TestCloseIsIdempotentAndReleasesPorts

10. Update environment tests for `PULSE_LOCAL_PROXY`, `PULSE_LOCAL_PROXY_HTTP_PORT`, `PULSE_LOCAL_PROXY_HTTPS_PORT`, `PULSE_LOCAL_PROXY_SKIP_TRUST_INSTALL`, `PULSE_FRONTEND_ADDR`, and `PULSE_DISABLE_FRONTEND_PROXY`.
11. Delete `internal/localproxy/caddyimports.go`.
12. Remove direct Caddy requirement by running `go mod tidy`; do not manually remove unrelated dependencies.
13. Update `docs/plans/0004-pulse-native-localproxy.md` progress and discoveries as each milestone lands.

## Validation and Acceptance

Focused validation during implementation:

    go test ./internal/localproxy
    go test ./cmd/pulse ./runtimeapp

Dependency validation:

    rg 'github.com/caddyserver' internal/localproxy go.mod go.sum
    go list -m all | rg 'caddy|certmagic|zerossl'

The first command should find no `internal/localproxy` Caddy imports and no direct Caddy requirement in `go.mod`. The second command should return no Caddy, CertMagic, or ZeroSSL modules unless another non-localproxy dependency genuinely still requires them.

Full repository validation before finishing:

    gofmt -w internal/localproxy/*.go cmd/pulse/dev_supervisor.go runtimeapp/app.go
    go test ./...
    go install ./cmd/pulse
    pulse harness self --json --write

Manual smoke validation when practical:

    PULSE_LOCAL_PROXY=1 PULSE_LOCAL_PROXY_SKIP_TRUST_INSTALL=1 PULSE_LOCAL_PROXY_HTTPS_PORT=9443 pulse run

Run this from a fixture app or a read-only app root. Confirm that the banner still prints HTTPS API, dashboard, MCP SSE, frontend, and DB Studio URLs as applicable. Also verify that `PULSE_LOCAL_PROXY=0 pulse run` avoids starting the proxy, that custom `PULSE_LOCAL_PROXY_HTTPS_PORT` appears in route URLs, and that explicit proxy hosts from `pulse.app` override workspace-derived hosts.

Acceptance criteria:

- `internal/localproxy` no longer imports any `github.com/caddyserver` packages.
- `internal/localproxy/caddyimports.go` is deleted.
- `go.mod` no longer directly requires `github.com/caddyserver/caddy/v2`.
- Existing normalization, route URL, workspace, and frontend discovery tests pass.
- New integration tests prove HTTPS reverse proxy parity for API, console, MCP, frontend, `/__pulse/config`, frontend `Host` rewrite, unknown host `404`, HTTP redirect, and `Close` lifecycle.
- TLS certificates contain the expected SAN DNS names and work with clients that trust the generated Pulse local CA.
- Trust installation is implemented and testable through mocks without modifying the real system trust store.
- `go test ./...` and `go install ./cmd/pulse` pass.

## Idempotence and Recovery

Certificate generation must be safe to rerun. Reusing an existing valid CA and leaf should not rewrite files. Regenerating an invalid, expired, or incomplete leaf should leave the CA intact unless the CA itself is unusable. If a write fails partway through, the next run should be able to replace the incomplete file set.

Trust installation should be idempotent. Re-running `Start` with `SkipInstallTrust == false` should not create duplicate trust entries or fail solely because the CA was already trusted. Tests must use the injectable trust installer and must not modify the real user or system trust store.

Listener startup should fail cleanly. Do not start any listener until config validation and certificate preparation have succeeded. If HTTPS listener creation fails, return an error and leave no goroutines running. If HTTP redirect listener creation fails after HTTPS listener creation, prefer parity with current Caddy binding behavior; unless investigation records otherwise, fail startup and close the HTTPS listener.

`Close` should be safe after partial startup failures and safe to call more than once. It should not call package-global stop functions and should not affect other `Proxy` instances.

## Artifacts and Notes

Default host naming for workspace `onlv`:

    api.onlv.localhost
    console.onlv.localhost
    mcp.onlv.localhost
    pulse.onlv.localhost

Route behavior to preserve:

- API host routes to `APIUpstream`, preserves incoming `Host`, and exposes `Routes.APIURL`.
- Console host routes to `DashboardUpstream`, preserves incoming `Host`, and exposes `Routes.ConsoleURL` only when dashboard routing is enabled.
- MCP host routes to `DashboardUpstream`, preserves incoming `Host`, and exposes `Routes.MCPBaseURL` only when dashboard routing is enabled.
- Frontend host routes exact path `/__pulse/config` to `APIUpstream` and preserves incoming `Host`.
- Frontend host routes all other paths to `FrontendUpstream`, rewrites `Host` to the frontend upstream host:port, and exposes `Routes.FrontendURL` only when frontend routing is enabled.

Exact startup validation errors to preserve:

    local proxy requires an API upstream
    local proxy requires an API host or workspace label
    local proxy requires console and mcp hosts when dashboard routing is enabled
    local proxy requires a frontend host when frontend routing is enabled

Important helper behavior to preserve:

    normalizeUpstream("") -> ""
    normalizeUpstream("0.0.0.0:4000") -> "127.0.0.1:4000"
    normalizeUpstream(":4000") -> "127.0.0.1:4000"
    normalizeUpstream("http://127.0.0.1:5178") -> "127.0.0.1:5178"
    normalizeHost("HTTPS://API.ONLV.LOCALHOST/path") -> "api.onlv.localhost"
    normalizeHost("api.onlv.localhost:443") -> "api.onlv.localhost"
    DiscoverWorkspace("/tmp/Onlv Repo", "fallback") -> "onlv-repo"

## Interfaces and Dependencies

No new routing, TLS, reverse proxy, or certificate dependency should be added. Use the Go standard library unless a later discovery proves a narrow external dependency is necessary and records the reason in the Decision Log.

Suggested internal files:

    internal/localproxy/proxy.go
    internal/localproxy/routes.go
    internal/localproxy/cert.go
    internal/localproxy/trust.go
    internal/localproxy/trust_darwin.go
    internal/localproxy/trust_linux.go
    internal/localproxy/trust_windows.go
    internal/localproxy/trust_other.go
    internal/localproxy/proxy_test.go
    internal/localproxy/cert_test.go

Environment variables to preserve:

    PULSE_LOCAL_PROXY
    PULSE_LOCAL_PROXY_HTTP_PORT
    PULSE_LOCAL_PROXY_HTTPS_PORT
    PULSE_LOCAL_PROXY_SKIP_TRUST_INSTALL
    PULSE_FRONTEND_ADDR
    PULSE_DISABLE_FRONTEND_PROXY
    PULSE_DEV_CACHE_DIR

URL helper contracts must not change:

    ConsoleAppURL(routes, appID) = routes.ConsoleURL + "/" + url.PathEscape(appID)
    MCPSSEURL(routes, appID) = routes.MCPBaseURL + "/sse?appID=" + url.QueryEscape(appID)
