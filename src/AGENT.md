# AGENT.md

- How to write code: [GUIDELINES.md](GUIDELINES.md)
- What it does: [docs/PRODUCT.md](docs/PRODUCT.md)
- What it must prove: [docs/TESTCASES.md](docs/TESTCASES.md)
- Why it is shaped this way: [docs/DECISIONS.md](docs/DECISIONS.md)
- How to build and demo: [README.md](../README.md)

## Layout

```
agentic_orchestration_framework/
  Makefile                    check, test, fmt, run (server + UI), run-sandbox (Docker, gh demo), ui, sandbox-image
  src/
    docs/                     product, test cases, decisions
    server/                   the Go project
      cmd/server/             config, the /v1 API, /demo/echo, the egress listener (path form and CONNECT proxy), the UI's JWT, and the adapters that join domains
      internal/agents/        agents by ID, versions by number
      internal/tools/         the tool registry: HTTP tools and sandboxed commands
      internal/executions/    the agent loop and its steps; deps/mock is the scripted model
      internal/gateway/       every tool call: checks, then an HTTP request or a sandboxed command
      internal/credentials/   the tenant token, the real credentials, and placeholders
      internal/sandbox/       one sandbox per execution; deps/local (processes) and deps/docker
      internal/egress/        exchanges a placeholder for the real credential on the way out; the platform CA for the proxy form
    ui/                       only what the demo needs; one folder per domain under src/domains/
```

Each domain has `<domain>.go` (what it is, its errors and `API`), `types.go`, `deps.go` (what it needs), `core.go` (pure), `service.go` (sequencing) and `deps/memory/`. Domains never import each other; `cmd/server/wire.go` adapts one domain's API to another's deps.

## A tool call

execution loop (pinned agent version, key `execution:turn:index`) → gateway: tool → grant → schema → allowlist → journal (a saved result comes back as is) → approval on file (else the loop waits for a human) → budget → then one of:

- **http**: the request with the real credential added → the response with the credential hidden
- **exec**: a placeholder issued for the call → the command in the sandbox, with arguments and the placeholder in its environment → its requests through egress (as a URL prefix, or as its HTTPS proxy with the platform CA), which exchanges the placeholder for the real credential for the tool's hosts only → the placeholder retired

→ a step of the execution
