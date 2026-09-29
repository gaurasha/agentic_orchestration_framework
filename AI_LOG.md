# AI usage log

## Three places I overrode AI

**1. Time-based credentials in the sandbox**
- **Asked:** given a tool gateway that stores all of a tenant's credentials, what issues come up when sandboxed execution runs commands like the `gh` CLI, and how can they be solved?
- **Produced:** the CLI needs a token in the pod, and its traffic is encrypted with TLS, so nothing outside can swap the token unless the pod trusts that party's CA. Its answer was a time-based approach: the real token is in the pod only during setup and teardown, not while the agent works.
- **What I did:** chose a credential swap at the egress proxy. The time-based token still entered the sandbox (a planted git hook could read it at teardown), and it didn't cover calls the agent makes mid-task. With the swap, the real credential never enters the sandbox: the pod trusts the platform's CA, so the proxy can open the request, swap the placeholder for the real token (or re-sign it) and send it on.

**2. Tool call validation**
- **Asked:** how the gateway should validate a tool call.
- **Produced:** validation against the tool registry alone: the tool exists and the arguments match its schema.
- **What I did:** made the agent's pinned version the authority. It grants each tool and sets the allowed argument values, for example `city` only London or Paris. The tool definition says what the tool is; the agent definition says how this agent may use it, so one tool can serve many agents with different limits. The architecture and the PoC's gateway both work this way.

**3. A per-call token checked against Temporal**
- **Asked:** how a shared worker proves it may call a tool for a run.
- **Produced:** the control plane signs a token after asking Temporal whether the call is pending.
- **What I did:** dropped it. Workers schedule the calls it checks, so it can't stop a compromised worker, and it adds a hop to every call. Instead, workers identify themselves to the gateway with a service-to-service token, and the gateway checks each call against the run's pinned version.

## Where AI was clearly better

**4. gVisor research**
- **Asked:** compare containers, gVisor, Kata/Firecracker, WebAssembly and remote executors for running arbitrary CLIs.
- **Produced:** pros, cons and sources for each, after its first answer (Docker with seccomp) was rejected.
- **What I did:** chose gVisor, with microVMs as the upgrade per tenant.

## What I did without AI

These are the decisions being graded, and the ones I have to defend. AI filled in details and reviewed; it didn't make these calls.

**5. Overall architecture diagram.** Drawn by hand in Excalidraw. Drawing it made me place each component and trust boundary myself. AI reviewed it only afterwards (#13).

**6. Key decisions and trade-off positions.** The five key decisions, and my position on each of the brief's four trade-offs. AI reviewed them after I wrote them.

**7. Guidelines, structure and scope for the PoC.** The coding rules come from me: portable domain folders, pure logic kept apart from I/O, and in-memory adapters. I set the folder layout, which part is built for real (the tool path) and what is mocked.

## Tools and models

**8.** Claude Code in VS Code (Claude Opus 5, then Opus 5.5); Excalidraw, by hand; Mermaid CLI; `go test`.

## More entries

**9. Two kinds of sandbox (overrode)**
- **Asked:** how execution tools get a sandbox.
- **Produced:** a warm pod per run, plus a shared executor for one-shot tools.
- **What I did:** kept one model, one warm pod per run. A second mode complicates the design and can be added later as an optimisation.

**10. Full transcripts in Temporal history (corrected)**
- **Asked:** where a run's trajectory is stored.
- **Produced:** Temporal history as the full record.
- **What I did:** moved large payloads to the object store, with a reference in history. Temporal's size limits and retention rule out full transcripts.

**11. Tools defined in code (overrode)**
- **Asked:** a tool registry.
- **Produced:** tools in code, registered at startup.
- **What I did:** made tools data, so tenants add tools without a release. A tool's definition fixes its host. Tools defined in code still fit system tools that rarely change.

**12. Resetting a shared pool (AI sharpened my idea)**
- **Asked:** should "better cleanup and checks" allow shared executor pools?
- **Produced:** cleanup can't prove a pod is clean; a reset to a clean snapshot can.
- **What I did:** used the snapshot condition.

**13. A review of my diagram (accepted)**
- **Asked:** review my first architecture diagram.
- **Produced:** fixes, such as missing arrows between components, and clearly showing the trust boundaries, such as which parts hold no credentials.
- **What I did:** took them all.

**14. A token echoed back into the sandbox (AI better)**
- **Asked:** review my tool call path.
- **Produced:** the pod sees a third party's response before the gateway redacts it, so an echoed token reaches untrusted code.
- **What I did:** added response scanning at the egress proxy.

**15. Hash-based agent versioning (kept)**
- **Asked:** how to version agents so that edits never change a running one.
- **Produced:** immutable definitions identified by a content hash, with each run pinned to one.
- **What I did:** kept it. Edits can't touch runs in flight, and the audit log names the exact version.

**16. Weighted fairness (AI improved my idea)**
- **Asked:** review my diagram, where Temporal's task queues and the LLM proxy each shared capacity between tenants in their own way.
- **Produced:** use the same fairness logic in both: the tenant as the key and its plan as the weight, for Temporal's fair queues (workers) and for token buckets in the LLM proxy (tokens).
- **What I did:** adopted it, so workers and model access are shared between tenants the same way.
