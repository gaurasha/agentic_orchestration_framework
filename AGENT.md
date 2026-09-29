# AGENT.md

Guidance for an AI coding agent working in this repository.

This is the tool call execution layer of a multi-tenant AI agent platform. Start at [src/AGENT.md](src/AGENT.md). It links the rules ([GUIDELINES.md](src/GUIDELINES.md)), the scope ([PRODUCT.md](src/docs/PRODUCT.md)), what must be proven ([TESTCASES.md](src/docs/TESTCASES.md)) and the reasons ([DECISIONS.md](src/docs/DECISIONS.md)). Read GUIDELINES.md before writing code. Its rules are binding, and each one gives its reason.

## Status

The ten demos in the README run end to end, one seeded agent and run per part of the design (`01-credential-hidden` to `10-gh-tls-intercept`): agent versioning, HTTP tool calls through the gateway with the credential added and hidden, sandboxed commands that hold only a placeholder and reach out through egress (as a URL prefix, or as an HTTPS proxy with the platform CA for `gh`), approvals/questions/cancel, idempotent retries, loop detection, and tenant isolation. Everything is in memory; a restart wipes it. Every case in TESTCASES.md has a linked test.

## Commands

Run these from the repo root. The Go module is `go 1.25`; the UI is Vite + React 19 + TypeScript.

```bash
make check   # gofmt check, go vet, go test, UI typecheck: run before calling work done
make test    # Go tests only
make fmt
make run     # builds .build/server, runs it (API :8080, egress :8081, local sandbox, seeded for acme) plus the Vite UI on :5173; Ctrl-C stops both. Server env: ADDR, EGRESS_ADDR, EGRESS_URL, EGRESS_CA_FILE, SANDBOX=local|docker, SANDBOX_IMAGE, SEED=<tenant>|none, GH_CLI_TOKEN
make run-sandbox    # the same with SANDBOX=docker and EGRESS_URL=http://192.168.64.1:8081 (Colima's bridged address); checks Colima has an address, builds the image if missing; GH_CLI_TOKEN=<token> seeds github_token, a `github` agent and one background run of it
make sandbox-image  # builds the Docker sandbox image
make ui      # the Vite dev server alone (proxies /v1 to API_URL or localhost:8080)
```

Run a single test:

```bash
cd src/server && go test ./internal/gateway -run TestRefusals -v
```

## Architecture

**Domains never import each other.** Each `src/server/internal/<domain>/` declares its own `API`, its own sentinel errors and, in `deps.go`, the interfaces it needs from outside. `cmd/server/wire.go` is the only place that joins domains: `newApp` builds every domain on its `deps/memory` adapter, and small adapter structs satisfy one domain's dep interface by calling another's `API`, with compile-time assertions. Adapters also translate across the boundary: `tools.ErrNotFound` → `gateway.ErrDenied`, and a `gateway.ErrDenied`/`ErrExhausted` → an `executions.ToolResult` with `Outcome: Refused`, so the model reads the reason. A new cross-domain need goes: an interface in the consumer's `deps.go`, then an adapter in `wire.go`. Never an import.

**`cmd/server` is the only place that reads the environment.** `api.go` holds the `/v1` handlers and `/demo/echo` (the fake external API the demo tool calls). `seed.go` loads the README's ten demos for `SEED` (default `acme`) through the domain APIs once both listeners are bound, each run with a description saying what to expect; the sandbox run (demo 9) only with `SANDBOX=local`, the gh run (demo 10) only with `GH_CLI_TOKEN` under Docker, in the background; `TestSeed` keeps it working. `auth.go` holds the tenant JWT middleware; `tenantOf(ctx)` returns `(tenant, ok)` so a handler outside the middleware can never act as tenant `""`. Domains take clock, IDs, config and loggers as deps.

**The tool call path** is `executions` → `gateway.API.Call`, checked in a fixed order: tool from the registry → grant from the *pinned* agent version → JSON Schema (`schema.go`) → argument allowlist → journal by key (`execution:turn:index`; a saved result is returned with `Replayed`) → approval on file (`ErrApprovalNeeded` otherwise; `gateway.Approve` records one per key) → budget → then by kind. `http`: the request with the credential header added, the response with the credential replaced by `[secret]` and capped. `exec`: a placeholder issued via `credentials.IssuePlaceholder` (bound to tenant, execution, ref, hosts), the command run by `sandbox` with an environment of exactly `ARG_<name>`, `EGRESS` and the placeholder, then the placeholder retired. Refusals cost no budget. Redirects are off (`noRedirect` in wire.go).

**Egress** (`internal/egress`, served on its own listener by `cmd/server/egress.go`) is how a sandboxed command reaches the outside, in two forms behind one `Forward`: the path form `/{scheme}/{host}/{path}`, and the proxy form, where the sandbox has the same URL as `HTTPS_PROXY`/`http_proxy` and a `CONNECT host:443` is hijacked, answered with a per-host leaf from `egress.CA` (the platform CA, per process; its PEM is written to `EGRESS_CA_FILE` and given to the sandbox as `SSL_CERT_FILE`, mounted read-only by the Docker runtime), and each request inside the TLS connection is forwarded as `https`; a proxy request for a plain `http` URL is forwarded too, and one addressed to egress itself falls through to the path form. `:443`/`:80` are stripped so hosts match the tool's list as written. The domain finds `fake_…` placeholders in the request headers, resolves each via `credentials.ResolvePlaceholder(value, host)` (401 unknown/expired/retired, 403 wrong host) before anything leaves, forwards with the real value, and puts the placeholder back wherever the reply carries the real value. Placeholders are also found and swapped inside a `Basic` credential, so `git` and `curl -u` work; an unauthenticated request gets a `WWW-Authenticate` challenge so git retries with its credentials. Plain `http` upstreams are allowed only with `AllowHTTP`, on for the demo's local stand-ins. Replies are scanned for the real value plain and inside base64 runs (an echoed Basic header). `/demo/git/{tenant}/{ref}/…/info/refs` in `api.go` is a fake git server that advertises branches only to the real stored value; the seeded `git_refs_demo` runs the real `git ls-remote` against it, and `git_refs` does the same against github.com once a `github_token` credential is stored. Git commands pass `-c credential.helper=` so the host's keychain helper stays out of the sandbox.

**Sandbox runtimes**: `sandbox/deps/local` runs processes in a temp dir with only the given env plus PATH and HOME; `sandbox/deps/docker` runs one container per command on a per-execution volume, and the command reaches egress as `host.docker.internal`. Containers reach the host only when Colima was started with `colima start --network-address` (its `colima.yaml` still says `address: false`, so a plain restart loses it; it hosts two kind clusters, so ask before restarting). Even then `host.docker.internal` goes through Lima's user-mode network, which drops out intermittently here; the bridged address `192.168.64.1` (the Mac on `col0`) is reliable, so run with `EGRESS_URL=http://192.168.64.1:8081`. A Docker network on `192.168.64.0/20` shadows that address (an empty `cassandra_cassandra-net` did). Without a route an egress call from a container times out. On a Mac, Go binaries such as `gh` ignore `SSL_CERT_FILE`, so the `gh` demo needs the Docker sandbox; `curl` honours it, which is what the tests use. The end-to-end test in `cmd/server/api_test.go` uses the local runtime with real `sh` and `curl`, so `make check` needs no Docker.

**Secrets.** `credentials.Secret` prints `[secret]` under every `fmt` verb and in JSON. Call `Reveal()` only at the point of use (the gateway, when it sets the header). Test 3.3 in `cmd/server/api_test.go` fails if any API body or log line contains the value.

**The execution loop** (`executions/service.go`) keeps `Execution.History` and resumes from it: `runCalls` pauses at a `NeedsApproval` result (status `waiting`, `Wait{Kind: approval, Turn, Index}`), `loop` pauses at a model `Question`. `Decide` approves via the gateway then re-issues the same key, or records a `rejected` step; `Answer` appends a user message; `Cancel` ends a waiting run; `Retry` re-issues a step's key and appends a `replayed` step. `repeating` refuses the same call after `MaxRepeats`. `finish` releases the run's sandbox at every terminal state (optional `Sandbox` dep). The **mock model** (`executions/deps/mock`) splits the input at `ask` lines into segments and walks them by counting user messages, so a scripted run can call, ask, call, finish.

## Conventions

- Domain layout: `<domain>.go`, `types.go`, `deps.go`, `core.go` (pure, table tests, no mocks), `service.go` (sequencing, tested against `deps/memory`), `deps/memory/`. A domain's tests pass with only its own folder present; small stubs in the test file are fine, mocking frameworks are not.
- Domain structs carry JSON tags: the API serves them directly. `Tenant` is always `json:"-"`.
- When you write a test for a test case, link it in that case's Test column in TESTCASES.md, and put the case ID in a comment above the test.
- UI: one folder per domain in `src/ui/src/domains/<domain>/` with `types.ts`, `deps.tsx`, `http.ts` and pure components; a `<Domain>Page.tsx` holds the state. Only `App.tsx` knows more than one domain. Style with the tokens in `theme.css` only.
- Docs are terse, and every rule or decision states its reason. When a change alters scope or design, keep PRODUCT.md, TESTCASES.md and DECISIONS.md in sync.
