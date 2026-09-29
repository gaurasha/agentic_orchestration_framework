# Product

A platform that runs AI agents for many tenants, safely. It has eight parts:

1. Agent definition and versioning
2. Durable agent executions
3. Safe tool calls, credential management
4. HITL & approvals
5. Sandboxed execution
6. Fairness & LLM access (budgets and limits)
7. Agent audit and observability
8. Tenant isolation

Out of scope: the agent's intelligence, multi-agent, multi-region, auth, payments, user management and policy-based authz.

## Scope

It demonstrates the tool execution layer end to end, all in memory. What it does for each part:

| Part | What it does |
|---|---|
| 1 | An agent has an ID; each edit makes a new numbered version with a content hash, and a version never changes. Saving unchanged content makes no new version. Each version lists the tools the agent may use, with an allowlist of argument values and whether a human must approve each call, from the tool registry; a tool the registry lacks is refused. |
| 2 | A basic agent loop: call the model, run the tools it asks for, feed back the results, repeat until the final answer. It runs inside the request that starts or resumes it. Each call carries a key, `execution:turn:index`, so a retry gets the saved result and nothing runs twice. The same call repeated N times is refused with a message to the model; a step cap ends a run that never answers. |
| 3 | Tools are data in a tool registry, of two kinds. An HTTP tool is a request template, with `{arg}` placeholders allowed in the path or query only, so an argument can never change the host; the gateway adds the real credential to the request and hides it in the response. An exec tool is a command; see part 5. Every call goes through the gateway, which checks in order: the tool, the pinned version's grant, the arguments against the tool's JSON Schema and the grant's allowlist, the journal (a saved result comes back as is), approval on file, then the budget. Redirects are off. No API ever returns a credential. |
| 4 | A call the version marks "needs approval" pauses the execution. The UI approves (the gateway records the approval for that one call and runs it), rejects (the reason goes to the model, and the run continues) or cancels. The model can ask the user a question; the run waits for the answer. An answer never counts as an approval. Each wait has a key that the decision or answer must name, so one meant for an earlier wait is refused rather than applied to the next. |
| 5 | An exec tool's command runs in the execution's sandbox: local processes, or Docker with `SANDBOX=docker`. Its environment holds only what the gateway gives it: each argument as `ARG_<name>`, the egress URL (also as its HTTP proxy), a placeholder where the credential goes, and the platform CA to trust. The command sends its requests through egress, which exchanges a live placeholder for the real credential on a request to one of the tool's hosts only, and puts the placeholder back in the reply. A tool that takes a base URL addresses egress as `$EGRESS/{scheme}/{host}/{path}`; a CLI that fixes its host, such as `gh`, reaches egress as its proxy, and egress answers the CONNECT with a certificate for that host signed by the platform CA, which only the sandbox trusts, and swaps inside the TLS connection. The placeholder is retired when the call ends. The real credential never enters the sandbox. Files persist across the run's commands, and the sandbox is released when the run ends. |
| 6 | The model is a scripted mock. The gateway enforces each execution's tool call budget; refusals and replays cost nothing. |
| 7 | A run may carry a description, a human's note on what it is for and what to expect, shown with it and never sent to the model; the seeded runs use it to say how to read their steps. Each execution keeps its steps, saved as each completes so a running execution shows its progress: every tool call, its arguments, what the model saw back, the exit code of a command, whether it was approved or replayed, and each question with its answer. The gateway logs each call before and after it runs, each refusal with its reason, and each approval; egress logs each outbound request and its decision. |
| 8 | An endpoint mints a JWT carrying the tenant ID; the UI sends it with every request, and everything a request does is scoped to that tenant. Journal entries, approvals and placeholders are keyed by tenant. |

Also:

- An execution stays on the agent version it started with.
- A refusal goes back to the model with its reason, and the execution continues.
- Tool calls and commands time out; a tool that fails returns an error result to the model. A call whose reply was lost after it was sent is an uncertain result: saved under its key and replayed on retry, never sent again.
- The mock model reads the input one line at a time: `tool {json args}` calls a tool, `ask <question>` asks the user and waits. Every demo run is the same.
- `/demo/echo` is a fake external API built into the server, which reports the credential header it received and numbers each request, so the demos prove the credential arrived and that a replay sent nothing.

What it must prove: [TESTCASES.md](TESTCASES.md).

## Later

- A sandbox network that allows only egress; today a command that ignores `$EGRESS` can reach the network directly (with no credential).
- Durable storage, a real model provider and a workflow engine, behind the same interfaces. A run then survives a restart and can be cancelled while a call runs.
- Rate limits and token budgets per tenant.

## The UI

Minimal, for the demo: store credentials and tools of both kinds, create and edit an agent, run it, approve, reject, answer or cancel when it waits, retry a step, and follow each run's steps and the version it ran on.
