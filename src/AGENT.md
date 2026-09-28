# AGENT.md

- How to write code: [GUIDELINES.md](GUIDELINES.md)
- What it does: [docs/PRODUCT.md](docs/PRODUCT.md)
- What it must prove: [docs/TESTCASES.md](docs/TESTCASES.md)
- Why it is shaped this way: [docs/DECISIONS.md](docs/DECISIONS.md)
- How to build: [README.md](../README.md)

## Layout

```
agentic_orchestration_framework/
  Makefile                    check, test, fmt
  src/
    docs/                     product, test cases, decisions
    server/                   the Go project
      internal/gateway/       every tool call and outbound request
      internal/sandbox/       one sandbox per execution
      internal/credentials/   caller tokens and real credentials
      internal/audit/         hash-chained audit log
    ui/                       only what the demo needs
```

Each domain has `<domain>.go` (what it is, its types and `API`) and `deps.go` (what it needs). Domains never import each other; `server/cmd/` (not yet written) wires them.

## A tool call

token → tool → grant, allowlist, budget → idempotency key → audit → run (HTTP from the gateway, or a command in the sandbox) → real credential added to the external request → audit → result
