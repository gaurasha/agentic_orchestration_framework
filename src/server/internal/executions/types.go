package executions

import (
	"encoding/json"
	"time"
)

type Status string

const (
	Running   Status = "running"
	Waiting   Status = "waiting" // on a human; see Execution.Wait
	Succeeded Status = "succeeded"
	Failed    Status = "failed"
	Cancelled Status = "cancelled"
)

// Run is what starts an execution. Description is a human's note on the
// run, what it is for and what to expect, shown with it and never sent to
// the model.
type Run struct {
	AgentID     string
	Input       string
	Description string
}

type Execution struct {
	ID           string    `json:"id"`
	Tenant       string    `json:"-"`
	AgentID      string    `json:"agentId"`
	AgentVersion int       `json:"agentVersion"` // pinned for the whole run
	Input        string    `json:"input"`
	Description  string    `json:"description,omitempty"`
	Status       Status    `json:"status"`
	Wait         *Wait     `json:"wait,omitempty"` // while Waiting
	Steps        []Step    `json:"steps"`
	Output       string    `json:"output"` // the final answer
	Error        string    `json:"error,omitempty"`
	StartedAt    time.Time `json:"startedAt"`
	EndedAt      time.Time `json:"endedAt"`
	History      []Message `json:"-"` // the conversation so far, to resume from
}

type WaitKind string

const (
	WaitApproval WaitKind = "approval"
	WaitQuestion WaitKind = "question"
)

// Wait is what a waiting execution needs from a human. Key names this
// wait; a decision or an answer must carry it, so one meant for an earlier
// wait is refused instead of landing on the next.
type Wait struct {
	Key      string          `json:"key"`
	Kind     WaitKind        `json:"kind"`
	Tool     string          `json:"tool,omitempty"` // WaitApproval: the pending call
	Args     json.RawMessage `json:"args,omitempty"`
	Question string          `json:"question,omitempty"` // WaitQuestion
	AskedAt  time.Time       `json:"askedAt"`
	Turn     int             `json:"-"` // WaitApproval: where to resume
	Index    int             `json:"-"`
}

type Decision struct {
	Key     string `json:"key"` // the wait it decides, from Wait.Key
	Approve bool   `json:"approve"`
	Reason  string `json:"reason"` // shown to the model on rejection
}

// Reply answers a question.
type Reply struct {
	Key  string `json:"key"` // the wait it answers, from Wait.Key
	Text string `json:"text"`
}

// Outcome is how one step went.
type Outcome string

const (
	OK       Outcome = "ok"
	Errored  Outcome = "error"    // the tool ran and failed
	Refused  Outcome = "refused"  // the gateway did not run it; Output says why
	Rejected Outcome = "rejected" // a human said no; Output is the reason
	// NeedsApproval is a gateway result only: the loop pauses instead of
	// recording a step.
	NeedsApproval Outcome = "needs_approval"
)

// Step is one tool call, as the model saw its result, or one question and
// its answer.
type Step struct {
	Index    int             `json:"index"` // from 1
	Tool     string          `json:"tool"`
	Kind     string          `json:"kind,omitempty"` // "http", "exec" or "question"; empty when refused
	Args     json.RawMessage `json:"args"`
	Outcome  Outcome         `json:"outcome"`
	ExitCode *int            `json:"exitCode,omitempty"` // exec only
	Output   string          `json:"output"`             // never a secret
	Replayed bool            `json:"replayed,omitempty"` // a retry served from the journal
	Approved bool            `json:"approved,omitempty"` // ran after a human approved it
	At       time.Time       `json:"at"`
	Key      string          `json:"-"` // the call's idempotency key
}

type Config struct {
	MaxSteps    int           // model calls before the execution fails
	MaxRepeats  int           // identical calls before the loop refuses to repeat one
	ToolTimeout time.Duration // per tool call; 0 means none
}

// Agent is what the loop needs of one agent version.
type Agent struct {
	ID           string
	Version      int
	Model        string
	Instructions string
	Tools        []Tool
}

type Tool struct {
	Name        string
	Description string
	Params      json.RawMessage // JSON Schema
}

// Caller names the execution to the gateway, with its pinned version.
type Caller struct {
	Tenant       string
	Execution    string
	AgentID      string
	AgentVersion int
}

type Prompt struct {
	Model   string
	System  string
	History []Message
	Tools   []Tool
}

type Message struct {
	Role       string     `json:"role"` // "user", "assistant" or "tool"
	Text       string     `json:"text,omitempty"`
	ToolCalls  []ToolCall `json:"toolCalls,omitempty"`  // assistant; none means the final answer
	Question   string     `json:"question,omitempty"`   // assistant: asks the user and waits
	ToolCallID string     `json:"toolCallId,omitempty"` // tool
	IsError    bool       `json:"isError,omitempty"`    // tool
}

type ToolCall struct {
	ID   string
	Name string
	Args json.RawMessage
}

type ToolResult struct {
	Output   string
	Outcome  Outcome
	Kind     string // "http" or "exec"; empty when refused
	ExitCode *int   // exec only
	Replayed bool
}
