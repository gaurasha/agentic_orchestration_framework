package gateway

import (
	"encoding/json"
	"time"
)

// Caller is who makes the call: the execution loop, on a pinned version.
type Caller struct {
	Tenant       string
	Execution    string
	AgentID      string
	AgentVersion int
}

type Call struct {
	Caller Caller
	Tool   string
	Args   json.RawMessage // a JSON object; null or empty means none
	// Key names the call, e.g. execution:turn:index. A repeat with the same
	// key gets the saved result instead of running again. Empty means no
	// journaling.
	Key string
}

// Result is what the model sees. The credential never appears in it.
type Result struct {
	Kind      Kind
	Status    int    // KindHTTP: the HTTP status
	ExitCode  int    // KindExec: the command's exit code
	Output    string // the response body or the command's output, credential hidden, size capped
	IsError   bool   // the tool ran and failed: a non-2xx status or a non-zero exit
	Uncertain bool   // the call was sent but its reply was lost; its effect is unknown
	Replayed  bool   // served from the journal; nothing ran
}

type Kind string

const (
	KindHTTP Kind = "http" // the gateway makes the request and adds the credential
	KindExec Kind = "exec" // a command in the sandbox, which holds only a placeholder
)

// Tool is what the gateway needs to run one tool.
type Tool struct {
	Name       string
	Kind       Kind
	Params     json.RawMessage // JSON Schema of the arguments; empty means unchecked
	Method     string          // KindHTTP
	URL        string          // KindHTTP: {arg} placeholders in the path or query
	Argv       []string        // KindExec
	Timeout    time.Duration   // KindExec; 0 means Config.ExecTimeout
	Credential *Credential
}

// Credential names a stored credential and where the tool receives it.
type Credential struct {
	Ref    string
	Header string   // KindHTTP: carries the real value
	Prefix string   // KindHTTP
	Env    string   // KindExec: carries a placeholder
	Hosts  []string // KindExec: where the placeholder may be exchanged
}

// Grant is what a caller may do, from its pinned agent version.
type Grant struct {
	Tools  map[string]ToolGrant
	Budget Budget
}

type ToolGrant struct {
	Allowlist     map[string][]string // argument -> allowed values; empty allows any
	NeedsApproval bool
}

// Budget caps an execution's tool calls; 0 means no cap.
type Budget struct {
	MaxCalls int
}

// Command is what the sandbox runs. Env is the whole environment.
type Command struct {
	Argv    []string
	Env     map[string]string
	Timeout time.Duration
}

type Output struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type Config struct {
	MaxOutput      int           // bytes of output the model may see
	Egress         string        // the URL a sandboxed command uses to reach the egress endpoint, given as EGRESS and as its HTTP proxy
	ExecTimeout    time.Duration // default per command
	PlaceholderTTL time.Duration // a placeholder's life beyond its call, as a backstop to retirement
}
