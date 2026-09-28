package agents

import "time"

type Definition struct {
	Name         string
	Instructions string // the system prompt
	Model        string
	Tools        []ToolGrant
	Budget       Budget
}

// ToolGrant is a tool the agent may use, and on what terms.
type ToolGrant struct {
	Name          string
	Allowlist     map[string][]string // argument -> allowed values; unlisted arguments are refused
	NeedsApproval bool
}

// Budget caps one execution's tool calls.
type Budget struct {
	MaxCalls int
}

// Agent is one version of an agent.
type Agent struct {
	Definition
	ID        string
	Version   int // from 1
	Tenant    string
	Hash      string // of the definition, to spot an unchanged save
	CreatedAt time.Time
}
