# AGENT.md

- How to write code: [GUIDELINES.md](GUIDELINES.md)
- What it does: [docs/PRODUCT.md](docs/PRODUCT.md)
- What it must prove: [docs/TESTCASES.md](docs/TESTCASES.md)
- Why it is shaped this way: [docs/DECISIONS.md](docs/DECISIONS.md)
- How to build: [README.md](../README.md)

## Layout

```
agentic_orchestration_framework/
  Makefile                    check, test, fmt, run
  src/
    docs/                     product, test cases, decisions
    server/                   the Go project
      cmd/server/             config, HTTP, the UI's JWT, and the adapters that join domains
      internal/agents/        agents by ID, versions by number
      internal/tools/         the tool registry
      internal/executions/    the agent loop, its workflow interface, HITL
      internal/llm/           the LLM proxy
      internal/gateway/       every tool call and outbound request
      internal/credentials/   caller tokens and real credentials
      internal/sandbox/       one sandbox per execution
      internal/timeline/      each execution's trajectory
    ui/                       only what the demo needs
```

Each domain has `<domain>.go` (what it is, its errors and `API`), `types.go` (its structs) and `deps.go` (what it needs). Domains never import each other; `cmd/server/wire.go` adapts one domain's API to another's deps.

## A tool call

token → tool → grant, allowlist, budget → idempotency key → run (HTTP from the gateway, or a command in the sandbox) → real credential added to the external request → result
