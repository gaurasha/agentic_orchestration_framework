package gateway

import (
	"encoding/json"
	"time"
)

type Call struct {
	Token          string // the caller's short-lived token
	Tool           string
	Args           json.RawMessage // a JSON object
	IdempotencyKey string          // unique per execution; a retry gets the first result
}

type Result struct {
	Output  string
	IsError bool // the tool ran and failed
}

// Caller is what a verified token says.
type Caller struct {
	Tenant       string
	Execution    string
	AgentID      string
	AgentVersion int // pinned for the execution
}

type Kind string

const (
	KindHTTP Kind = "http" // the gateway makes the request
	KindExec Kind = "exec" // a command in the execution's sandbox
)

type Tool struct {
	Tenant     string
	Name       string
	Kind       Kind
	Spec       json.RawMessage // an HTTP request template or a command
	Hosts      []string        // the only hosts its requests may reach
	Credential string          // reference to the credential it needs; may be empty
}

// Grant is what a caller may do, from its pinned agent version.
type Grant struct {
	Tools  map[string]Allowlist // tool -> its allowed arguments
	Budget Budget
}

// Allowlist maps each argument to its allowed values; unlisted ones are refused.
type Allowlist map[string][]string

type Budget struct {
	MaxCalls int
}

type Command struct {
	Argv    []string
	Env     map[string]string // never a real credential
	Timeout time.Duration
}

type Output struct {
	ExitCode int
	Stdout   string
	Stderr   string
}
