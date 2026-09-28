# Decisions

Why it is shaped this way.

| Decision | Why |
|---|---|
| Demonstrate the tool execution layer end to end ([PRODUCT.md](PRODUCT.md)) | it holds authorization, budgets, credentials and sandboxing |
| One gateway for every tool call and all egress | one place to enforce; nothing routes around it |
| Callers hold short-lived tokens; the real credential is added only at the external call | a leaked token soon expires; the real credential never leaves the gateway |
| One sandbox per execution; in production hibernated with a backup, network only to the gateway | files persist; an idle one costs storage, not a pod. Only one per execution is built for now |
| Tools are data | tenants add tools without a release |
| A minimal UI, only for the demo | it proves the tool layer; the UI only shows it |
| Stand-ins: SQLite; an in-memory workflow runner; a mocked LLM proxy; a sandbox that is mocked or runs on this machine | it proves the tool layer, not infrastructure; each sits behind an interface a production part replaces |
| No shared package and no `doc.go`: each domain declares its own errors, and its `<domain>.go` says what it is | a domain moves with nothing else |
| Packages: agents, tools, executions, llm, gateway, credentials, sandbox, timeline; `cmd/server` holds the UI's JWT check and the adapters that join them | one concern each |
| Cut for now: the audit log, sandbox hibernation and backups, and CLI requests returning through the gateway | not needed to prove the tool layer |
| Idempotency journal | Temporal retries activities; a retry must not repeat a side effect |

Open, for production: how a CLI's HTTPS request reaches the gateway for the swap (TLS termination, a platform CA).
