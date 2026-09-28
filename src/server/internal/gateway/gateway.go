// Package gateway is the tool gateway: the one path every tool call and
// every outbound request takes.
//
// For each call it verifies the caller's short-lived token, loads the tool
// definition, checks the grant, the argument allowlist and the budget,
// claims the idempotency key so a retry never runs the call twice, audits
// the decision, runs the tool, and adds the real credential only to the
// external request itself.
//
// Needs: the interfaces in deps.go.
// Portability: interfaces only so far; no logic, service or adapters yet.
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// Errors a refused call wraps; callers match them with errors.Is.
var (
	ErrUnauthenticated = errors.New("unauthenticated")  // token missing, forged or expired
	ErrDenied          = errors.New("denied")           // tool not granted, or an argument or host not allowed
	ErrExhausted       = errors.New("budget exhausted") // the execution's budget is used up
	ErrConflict        = errors.New("conflict")         // a call with the same key is still running
)

// API is what the gateway offers.
type API interface {
	// Call runs one tool call through the whole pipeline. A refused call
	// returns an error wrapping ErrUnauthenticated, ErrDenied or
	// ErrExhausted, with the reason in its message.
	Call(ctx context.Context, c Call) (Result, error)

	// Forward sends one outbound request from a sandbox. The request carries
	// the caller's short-lived token where the real credential would go.
	// Forward checks the host against the tools the execution may use,
	// swaps the token for the real credential, and sends the request.
	Forward(ctx context.Context, r *http.Request) (*http.Response, error)
}

// Call is one tool call, as the caller sends it.
type Call struct {
	Token          string          // the caller's short-lived token
	Tool           string          // the tool's name in the catalog
	Args           json.RawMessage // the arguments, as a JSON object
	IdempotencyKey string          // unique within the execution; a retry with it gets the first result
}

// Result is what a tool call returns.
type Result struct {
	Output  string
	IsError bool // the tool ran and failed; the caller should see why
}

// Caller is what a verified token says about who is calling.
type Caller struct {
	Tenant       string
	Execution    string
	AgentVersion string // content hash of the agent definition the execution is pinned to
}

// Kind is how a tool runs.
type Kind string

const (
	KindHTTP Kind = "http" // the gateway makes the request itself
	KindExec Kind = "exec" // a command in the execution's sandbox
	KindCLI  Kind = "cli"  // a command in the sandbox whose requests come back through Forward
)

// Tool is a tool definition. Tools are data in the catalog, not code.
type Tool struct {
	Tenant     string
	Name       string
	Kind       Kind
	Spec       json.RawMessage // how to run it: an HTTP request template or a command
	Hosts      []string        // the hosts its requests may reach
	Credential string          // reference to the credential its requests need; empty if none
}

// Grant is what one execution may do, taken from its pinned agent version.
type Grant struct {
	Tools  map[string]Allowlist // tool name -> the arguments it may take
	Budget Budget
}

// Allowlist maps each argument name to the values it may take.
// An argument that is not listed is refused.
type Allowlist map[string][]string

// Budget caps what one execution may spend on tool calls.
type Budget struct {
	MaxCalls int
}

// Command is a command to run in a sandbox.
type Command struct {
	Argv    []string
	Env     map[string]string // never a real credential
	Timeout time.Duration
}

// Output is what a command produced.
type Output struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Event is one audited step of a call: its decision, or its outcome.
type Event struct {
	At       time.Time
	Caller   Caller
	Tool     string
	Action   string // "call" or "forward"
	Decision string // "allow" or "deny"
	Reason   string // why it was denied, or what happened
}
