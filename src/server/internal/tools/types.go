package tools

import "encoding/json"

// Kind is how a tool runs.
type Kind string

const (
	KindHTTP Kind = "http" // the gateway makes the request
	KindExec Kind = "exec" // a command in the execution's sandbox
)

type Tool struct {
	Tenant      string
	Name        string
	Description string          // shown to the model
	Params      json.RawMessage // JSON Schema of the arguments, shown to the model
	Kind        Kind
	Spec        json.RawMessage // an HTTP request template or a command
	Hosts       []string        // the only hosts its requests may reach
	Credential  string          // reference to the credential it needs; may be empty
}
