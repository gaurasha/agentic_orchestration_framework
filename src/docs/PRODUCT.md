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

It demonstrates the tool execution layer end to end. What it does for each part:

| Part | What it does |
|---|---|
| 1 | Agent definitions in SQLite. An agent has an ID; each edit makes a new numbered version, and a version never changes. Each lists the tools the agent may use, from the tool registry. |
| 2 | A basic agent loop (call the model, run the tools it asks for, repeat) behind a workflow interface. It runs in memory, so a restart loses running executions; Temporal can replace it behind the same interface. |
| 3 | Tools are data in a tool registry. Every tool call goes through the tool gateway, which checks the caller's short-lived token, the grant, the arguments and the budget, then adds the real credential from the credential store to the external request only. Credentials are encrypted at rest and never returned or shown. |
| 4 | An execution can pause for a human: to approve or reject a tool call, or to answer a question. It resumes when the UI answers. |
| 5 | One sandbox per execution, mocked or run directly on this machine without isolation, behind the interface a real sandbox will implement. |
| 6 | The LLM proxy is mocked behind an interface, so a real key can be used later. The gateway enforces tool budgets. Fairness between tenants is out of scope. |
| 7 | Each execution's trajectory (every model call, tool call, approval and result) is recorded in SQLite. No separate audit log. |
| 8 | A basic endpoint mints a JWT carrying the tenant ID; the UI sends it with every request, and everything a request does is scoped to that tenant. The workflow gets a short-lived token to call the tool gateway. |

Also:

- A retried tool call returns its first result instead of running twice.
- Tool calls time out, and an execution can be cancelled from the UI.
- An execution stays on the agent version it started with.
- An HTTP tool reaches only the hosts its definition lists.
- A fake external service checks the credential it receives, so the demo proves the gateway added it.
- The mocked LLM plays a scripted conversation, so every demo run is the same.

What it must prove: [TESTCASES.md](TESTCASES.md). Parts 2, 5 and 6 and the demo are not tested.

## The UI

Minimal, for the demo: create an agent definition, start an execution, follow its full timeline, and answer HITL requests.
