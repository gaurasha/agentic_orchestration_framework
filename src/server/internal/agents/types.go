package agents

import "time"

type Definition struct {
	Name         string      `json:"name"`
	Instructions string      `json:"instructions"` // the system prompt
	Model        string      `json:"model"`
	Tools        []ToolGrant `json:"tools"`
	Budget       Budget      `json:"budget"`
}

// ToolGrant is a tool the agent may use, and on what terms.
type ToolGrant struct {
	Name          string              `json:"name"`
	Allowlist     map[string][]string `json:"allowlist,omitempty"` // argument -> allowed values; empty allows any
	NeedsApproval bool                `json:"needsApproval"`
}

// Budget caps one execution's tool calls; 0 means no cap.
type Budget struct {
	MaxCalls int `json:"maxCalls"`
}

// Agent is one version of an agent.
type Agent struct {
	Definition
	ID        string    `json:"id"`
	Version   int       `json:"version"` // from 1
	Tenant    string    `json:"-"`
	Hash      string    `json:"hash"` // of the definition, to spot an unchanged save
	CreatedAt time.Time `json:"createdAt"`
}
