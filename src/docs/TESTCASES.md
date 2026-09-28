# Test cases

What it must prove, by part of [PRODUCT.md](PRODUCT.md); IDs keep the part numbers. The Test column links each test once it exists. Parts 2 (durable executions), 5 (sandboxed execution) and 6 (fairness & LLM access) and the demo are not tested.

## 1. Agent definition and versioning

| ID | Behaviour | Test |
|---|---|---|
| 1.1 | Creating an agent gives it an ID at version 1; saving content identical to the latest version creates no new version. | — |
| 1.2 | Editing an agent creates the next version number and leaves the old version unchanged. | — |
| 1.3 | An execution stays on the version it started with, even after the agent is edited. | — |
| 1.4 | A definition that lists a tool missing from the tenant's registry is refused. | — |

## 3. Safe tool calls, credential management

| ID | Behaviour | Test |
|---|---|---|
| 3.1 | A call with a missing, forged or expired token is refused. | — |
| 3.2 | A call to a tool the agent's version does not grant is refused, with the reason. | — |
| 3.3 | A call with an argument outside its allowlist is refused. | — |
| 3.4 | A call past the execution's budget is refused. | — |
| 3.5 | An HTTP tool's request reaches the external service with the real credential, which the caller never sees. | — |
| 3.6 | An HTTP tool cannot reach a host its definition does not list. | — |
| 3.7 | A retry with the same idempotency key returns the first result, and the external service sees one request. | — |
| 3.8 | A tool added to the registry works for an agent that lists it, with no code change or restart. | — |
| 3.9 | Credentials are encrypted at rest: the SQLite file does not contain them in plain text. | — |
| 3.10 | No API response, UI view, timeline event or log line contains a credential. | — |

## 4. HITL & approvals

| ID | Behaviour | Test |
|---|---|---|
| 4.1 | A tool call that needs approval pauses the execution; nothing runs until someone decides. | — |
| 4.2 | Approval runs the call; rejection returns the reason to the model, and the execution continues. | — |
| 4.3 | An execution that asks a question resumes with the answer given in the UI. | — |
| 4.4 | Only the execution's tenant can approve, reject or answer. | — |

## 7. Agent audit and observability

| ID | Behaviour | Test |
|---|---|---|
| 7.1 | Every model call, tool call, approval, answer and result appears in the execution's timeline, in order. | — |

## 8. Tenant isolation

| ID | Behaviour | Test |
|---|---|---|
| 8.1 | A request without a valid JWT is refused. | — |
| 8.2 | Tenant A cannot list, read, run, approve or use tenant B's agents, executions, tools or credentials. | — |
| 8.3 | A short-lived token issued for one execution does not work for another execution or tenant. | — |
