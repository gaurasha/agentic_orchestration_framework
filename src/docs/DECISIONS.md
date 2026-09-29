# Decisions

Why it is shaped this way.

| Decision | Why |
|---|---|
| Demonstrate the tool execution layer end to end ([PRODUCT.md](PRODUCT.md)) | it holds authorization, budgets, credentials and sandboxing |
| One gateway for every tool call | one place to enforce; nothing routes around it |
| The caller never holds the credential; the gateway adds it at the external call and hides it in the response | the model, the steps and the UI can leak nothing they never had |
| Tools are data, of two kinds: an HTTP request or a command | tenants add tools without a release; a CLI needs a sandbox, an API call does not |
| `{arg}` placeholders only in a URL's path or query; redirects off | an argument or a redirect can never send the credential to another host |
| A command's arguments arrive as `ARG_<name>` variables, never in the command line | an argument cannot inject into the command |
| The gateway checks in a fixed order: tool, grant, schema, allowlist, journal, approval, budget, then the call | a refusal or a replay costs no budget, and the reason names the first failed check |
| A refusal goes back to the model with its reason | the model can adapt; the demo shows why each call was refused |
| A sandboxed command gets a placeholder, and egress exchanges it for the real credential on the way out, for the tool's hosts only, until the call ends | the real credential never enters the sandbox, so planted code there cannot read it; a leaked placeholder is dead after the call and useless for any other host. Putting the real credential in the sandbox even briefly, at setup and teardown, was rejected for that reason |
| Egress has two forms behind one check-and-swap path. The path form: a plain HTTP endpoint the command addresses as `$EGRESS/{scheme}/{host}/{path}`. The proxy form: the sandbox has egress as its HTTP proxy, and a CONNECT is answered with a certificate for the host signed by a platform CA that only the sandbox trusts, so egress reads the request inside the TLS connection and swaps there. Both look for the placeholder inside a Basic credential too | the path form needs no CA and nothing to intercept, and serves `git`, `curl` and most SDKs, which take a base URL. A CLI that hard-codes its host, such as `gh`, cannot be pointed anywhere, so the proxy form opens its TLS connection instead; the pod trusting the platform's CA is what makes that possible, and nothing outside the sandbox trusts it. Either way the real credential never enters the sandbox. The CA's key lives in the server process and dies with it |
| A sandbox's environment is exactly what the gateway gives it, plus PATH, HOME and SSL_CERT_FILE naming the platform CA | nothing from the host leaks in; the CA is a public certificate, and the runtime, not the gateway, knows where it put it |
| Every call carries a key, `execution:turn:index`, and the gateway journals its result per tenant | a retry gets the saved result, so nothing acts twice; the journal is checked before approval and budget, so a replay asks for neither |
| A failure after the call was sent (the connection dropped, the reply cut short, a command timed out) is an uncertain result: journaled as an error, replayed on retry, never sent again | the effect of that call is unknown, and sending it again could act twice; a refusal or a failure before sending releases the key, since nothing happened. A human who knows the effect starts a new run |
| Each execution's transitions are serialized: a decision, an answer, a cancel or a retry holds the execution from read to save | two requests on one waiting run cannot both win, and a save cannot lose a step another request appended; a cancel that arrives during an approved call waits for it and is then refused, which is honest about what ran |
| A credential is hidden in every form a reply commonly carries it: plain, JSON-escaped, URL-escaped, and inside any base64 run, however short | replacing the literal value alone leaves it readable once a reply escapes a quote or backslash, or echoes a short Basic credential |
| A sandboxed command runs in its own process group, killed whole at the timeout, and its output is capped as it arrives | killing the shell alone leaves its children running and the pipes open; buffering before capping lets one noisy command take the server's memory |
| Egress keeps an escaped path segment as sent, and answers a reply over its size limit with an error | rebuilding the URL from the decoded path turns `%2F` into a separator; returning a cut body with the upstream's 200 hands a client broken data with no sign of it |
| Approval is a gateway check, on file per call key, not a flag the loop passes | the loop cannot skip it; an approval covers one call, never the tool |
| A question from the model and a decision on a call are different waits with different endpoints | a chat reply can never count as an approval |
| Every wait has a key, and a decision or an answer must carry it | two people approving the same call are serialized, and without the key the second approval would land on whatever the run waits for next, a call its author never saw |
| The loop runs inside the request that starts or resumes it, and keeps its history to resume from | no background jobs or workflow engine needed to pause for days; a workflow engine can replace it behind the same API |
| The same call repeated N times is refused with a message to the model | a looping agent stops burning budget, and the model is told why |
| Arguments are checked against the tool's JSON Schema and the grant's allowlist, in that order | the tool says what a valid call is; the version says what this agent may do with it |
| A minimal UI, only for the demo | it proves the tool layer; the UI only shows it |
| The server seeds the demos for `acme` at start, through the domain APIs (`SEED=none` to skip) | storage is in memory, so without it every restart means setting the demos up again by hand; the domain APIs make seeded data the same as data made in the UI |
| Stand-ins: in-memory stores; a scripted mock model; a signing key per process; a local-process sandbox by default, Docker on request | it proves the tool layer, not infrastructure; each sits behind an interface a production part replaces |
| No shared package and no `doc.go`: each domain declares its own errors, and its `<domain>.go` says what it is | a domain moves with nothing else |
| Packages: agents, tools, executions, gateway, credentials, sandbox, egress; `cmd/server` holds the API, the egress listener, the UI's JWT check and the adapters that join them | one concern each |
| Cut for now: a sandbox network that allows only egress, durable storage, a real model provider, a workflow engine, per-execution tokens, rate limits | not needed to prove the tool layer; the loop and the gateway run in one process |
