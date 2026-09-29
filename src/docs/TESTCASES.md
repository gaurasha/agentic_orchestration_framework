# Test cases

What it must prove, by part of [PRODUCT.md](PRODUCT.md); IDs keep the part numbers. The Test column links each test. Every case is built and tested; `make check` runs them all without Docker or the network.

## 1. Agent definition and versioning

| ID | Behaviour | Test |
|---|---|---|
| 1.1 | Creating an agent makes version 1; saving it unchanged makes no new version. | [TestCreateIsV1AndUnchangedSaveIsNoop](../server/internal/agents/service_test.go) |
| 1.2 | Editing an agent makes the next version and leaves earlier versions unchanged. | [TestEditMakesNextVersion](../server/internal/agents/service_test.go) |
| 1.3 | An execution stays on the version it started with, even after the agent is edited. | [TestExecutionPinsVersion](../server/internal/executions/service_test.go) |

## 3. Safe tool calls, credential management

| ID | Behaviour | Test |
|---|---|---|
| 3.1 | A call is refused, with the reason, when the pinned version does not grant the tool, an argument is outside its allowlist, or the budget is spent. A refused call never reaches the external service and costs no budget. | [TestRefusals](../server/internal/gateway/service_test.go) |
| 3.2 | An HTTP tool's request reaches the external service with the real credential, added by the gateway. | [TestCredentialAddedAndHidden](../server/internal/gateway/service_test.go), [TestCredentialReachesServiceButNothingElse](../server/cmd/server/api_test.go) |
| 3.3 | No API response, execution step or log line contains a real credential, even when the external service echoes it back, plain or in the encodings a reply commonly uses: JSON-escaped, URL-escaped, or inside a base64 run such as an echoed Basic credential, however short. | [TestCredentialReachesServiceButNothingElse](../server/cmd/server/api_test.go), [TestRedact](../server/internal/gateway/core_test.go), [TestHideEncodedForms](../server/internal/egress/core_test.go) |
| 3.4 | Each call carries a key (`execution:turn:index`, scoped to the tenant). A repeat with the same key gets the saved result and the external service sees one request; a refused call is not saved. A call whose reply was lost after it was sent is an uncertain result, saved like any other, so a retry replays it and never sends again. | [TestRetryReplaysSavedResult](../server/internal/gateway/service_test.go), [TestUncertainResultIsNotSentAgain](../server/internal/gateway/service_test.go), [TestRetryUsesTheSameKey](../server/internal/executions/service_test.go), [TestRetryIsReplayed](../server/cmd/server/api_test.go) |
| 3.5 | Arguments are checked against the tool's JSON Schema (required, types, enum, unknown fields) before anything runs. An argument is escaped for where it lands in the URL: a path segment or a query value. | [TestCheckSchema](../server/internal/gateway/schema_test.go), [TestSchemaRefusal](../server/internal/gateway/service_test.go), [TestBuildURLEscapesByPosition](../server/internal/gateway/core_test.go) |
| 3.6 | The gateway logs each call before and after it runs, and every refusal with its reason. | [TestCredentialReachesServiceButNothingElse](../server/cmd/server/api_test.go) |

## 4. HITL & approvals

| ID | Behaviour | Test |
|---|---|---|
| 4.1 | A tool call that needs approval pauses the execution; nothing runs until someone decides. The gateway itself refuses the call until an approval for its key is on file. | [TestApprovalOnFile](../server/internal/gateway/service_test.go), [TestApprovalPausesAndResumes](../server/internal/executions/service_test.go), [TestApprovalsQuestionsAndCancel](../server/cmd/server/api_test.go) |
| 4.2 | Approval runs the call, once, and the execution continues with the calls after it; rejection returns the reason to the model, and the execution continues. An approval is for one call: another call to the same tool needs its own. A decision names the wait it is for; one for a wait that has passed is refused, never applied to the next wait, and one without a key is invalid. | same, [TestStaleDecisionIsRefused](../server/internal/executions/service_test.go) |
| 4.3 | An execution that asks a question resumes with the answer given in the UI. An answer never counts as an approval, and a decision never counts as an answer. | [TestQuestionWaitsForAnswer](../server/internal/executions/service_test.go), [TestApprovalsQuestionsAndCancel](../server/cmd/server/api_test.go) |
| 4.4 | A waiting execution can be cancelled; it ends as cancelled and the pending call never runs. | [TestCancelWaiting](../server/internal/executions/service_test.go), [TestApprovalsQuestionsAndCancel](../server/cmd/server/api_test.go) |

## 5. Sandboxed commands

| ID | Behaviour | Test |
|---|---|---|
| 5.1 | A command in the sandbox never sees the real credential: its environment holds only the arguments, the egress URL (also as its HTTP proxy), a placeholder (`echo $GH_TOKEN` prints the placeholder) and the platform CA to trust, and nothing from the host. | [TestExecGetsPlaceholderOnly](../server/internal/gateway/service_test.go), [TestEnvIsOnlyWhatIsGiven](../server/internal/sandbox/deps/local/local_test.go), [TestSandboxedCommandUsesPlaceholder](../server/cmd/server/api_test.go) |
| 5.2 | A request from the sandbox reaches the external service with the real credential in place of the placeholder, for that tool's hosts only; a request to any other host is refused and carries no credential; a reply that echoes the credential carries the placeholder instead, even inside a base64 encoding. A placeholder inside a Basic credential, as `git` and `curl -u` send it, is found and swapped there; an unauthenticated probe gets a challenge so git retries with its credentials. An escaped path segment reaches the host as sent, and a reply over the size limit is an error, never a success with a cut body. | [TestEscapedPathAndOversizedReply](../server/internal/egress/service_test.go), [TestForwardSwapsAndHides](../server/internal/egress/service_test.go), [TestHideInsideBase64](../server/internal/egress/core_test.go), [TestBasicCredential](../server/internal/egress/core_test.go), [TestSandboxedCommandUsesPlaceholder](../server/cmd/server/api_test.go), [TestBasicCredentialThroughEgress](../server/cmd/server/api_test.go), [TestGitThroughEgress](../server/cmd/server/api_test.go) (a real `git` against a fake git server that answers only to the real value) |
| 5.3 | A placeholder stops working when its tool call ends or expires, and a forged one never works. | [TestPlaceholderLifetime](../server/internal/credentials/service_test.go), [TestSandboxedCommandUsesPlaceholder](../server/cmd/server/api_test.go) |
| 5.4 | Files a command writes are there for the run's next command; another run has its own sandbox; the sandbox is released when the run ends, not while it waits. Commands time out and report their exit code. | [TestSandboxKeepsFilesAcrossCommands](../server/cmd/server/api_test.go), [TestCancelWaiting](../server/internal/executions/service_test.go), [TestExitCodeAndTimeout](../server/internal/sandbox/deps/local/local_test.go) |
| 5.5 | A CLI that fixes its host, as `gh` does, reaches it through egress as its HTTP proxy: egress answers the CONNECT with a certificate for that host signed by the platform CA, which only the sandbox trusts; inside the TLS connection the placeholder is swapped and the reply hides the real value; another host is refused; the placeholder is dead after the call; a client that does not trust the CA gets nothing. | [TestLeafIsSignedForHost](../server/internal/egress/ca_test.go), [TestCLIThroughProxy](../server/cmd/server/api_test.go) (`curl` standing in for `gh`, which on a Mac ignores `SSL_CERT_FILE`; the real `gh` runs in the Docker sandbox, README demo 4) |

The end-to-end tests run a real `sh` and `curl` in the local sandbox against the server's own `/demo/echo`, over plain HTTP through the path form and over TLS through the CONNECT form.

## 6. Failure modes

| ID | Behaviour | Test |
|---|---|---|
| 6.1 | The same call repeated N times is refused with a message to the model instead of running again; a call is the same whatever the order of its argument keys. The step cap ends a run that never answers. | [TestRepeatedCallIsRefused](../server/internal/executions/service_test.go), [TestRepeating](../server/internal/executions/core_test.go) |
| 6.2 | A tool that fails or cannot be reached returns an error result to the model; the run continues. | [TestExecutionPinsVersion](../server/internal/executions/service_test.go) (the refused step), [TestSandboxKeepsFilesAcrossCommands](../server/cmd/server/api_test.go) (the failing `cat`) |
| 6.3 | Transitions of one execution never interleave: a decision, an answer, a cancel and a retry each hold the execution while they read, change and save it, so a cancel that arrives while an approved call is in flight waits and is then refused, and nothing a request appended is lost. | [TestDecideAndCancelCannotBothWin](../server/internal/executions/service_test.go) |
| 6.4 | A command that outlives its timeout is killed with every process it started, and the runtime returns at once. Output is bounded while it is collected, so a noisy command cannot grow the server's memory. | [TestTimeoutKillsTheProcessGroup](../server/internal/sandbox/deps/local/local_test.go), [TestCappedDropsExcessAsItArrives](../server/internal/sandbox/core_test.go) |

## 7. Audit and observability

| ID | Behaviour | Test |
|---|---|---|
| 7.1 | A running execution shows each step as soon as it completes, not only when the run ends. | [TestStepsAppearAsTheyComplete](../server/internal/executions/service_test.go) |

## 8. Tenant isolation

| ID | Behaviour | Test |
|---|---|---|
| 8.1 | Tenant A cannot list, read, run or use tenant B's agents, executions, tools or credentials; a request without a valid tenant token is refused. | [TestTenantIsolation](../server/cmd/server/api_test.go) |
