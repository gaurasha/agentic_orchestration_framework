package tools

import "encoding/json"

// Kind is how a tool runs.
type Kind string

const (
	KindHTTP Kind = "http" // the gateway makes the request and adds the credential
	KindExec Kind = "exec" // a command in the execution's sandbox, which holds only a placeholder
)

type Tool struct {
	Tenant      string          `json:"-"`
	Name        string          `json:"name"`
	Description string          `json:"description"`      // shown to the model
	Params      json.RawMessage `json:"params,omitempty"` // JSON Schema of the arguments, shown to the model
	Kind        Kind            `json:"kind"`
	HTTP        *HTTP           `json:"http,omitempty"` // KindHTTP
	Exec        *Exec           `json:"exec,omitempty"` // KindExec
	Credential  *Credential     `json:"credential,omitempty"`
}

// HTTP is the request template. {arg} placeholders may appear in the path
// or query only, so an argument can never change which host is called.
type HTTP struct {
	Method string `json:"method"`
	URL    string `json:"url"`
}

// Exec is the command. Arguments never go into Argv: they reach the
// command as environment variables ARG_<name>, so an argument cannot
// change the command line.
type Exec struct {
	Argv           []string `json:"argv"`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty"` // 0 means the gateway's default
}

// Credential names a stored credential and where the tool receives it.
// An HTTP tool gets the real value in Header. An exec tool gets a
// placeholder in Env, which the egress endpoint exchanges for the real
// value only on a request to one of Hosts.
type Credential struct {
	Ref    string   `json:"ref"`
	Header string   `json:"header,omitempty"`
	Prefix string   `json:"prefix,omitempty"` // e.g. "Bearer "
	Env    string   `json:"env,omitempty"`
	Hosts  []string `json:"hosts,omitempty"`
}
