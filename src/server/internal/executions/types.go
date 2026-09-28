package executions

import (
	"context"
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

type Execution struct {
	ID           string
	Tenant       string
	AgentID      string
	AgentVersion int // pinned for the whole run
	Input        string
	Status       Status
	Wait         *Wait  // while Waiting
	Output       string // the final answer
	Error        string // why it failed or was cancelled
	StartedAt    time.Time
	EndedAt      time.Time
}

// Wait is what a waiting execution needs: an approval (Tool set) or an
// answer (Question set).
type Wait struct {
	Tool     string
	Args     json.RawMessage
	Question string
}

type Decision struct {
	Approve bool
	Reason  string // shown to the model on rejection
}

type Config struct {
	MaxSteps    int           // model calls before the execution fails
	ToolTimeout time.Duration // per tool call; the model is told on timeout
	TokenTTL    time.Duration // of the token the loop gives the gateway
}

// Workflow is one execution's loop; decisions and answers arrive as signals.
type Workflow func(ctx context.Context, signals <-chan Signal) error

// Signal is a decision or an answer.
type Signal struct {
	Decision *Decision
	Answer   *string
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
	Name          string
	Description   string
	Params        json.RawMessage // JSON Schema
	NeedsApproval bool
}

type Prompt struct {
	Tenant    string
	Execution string
	Model     string
	System    string
	History   []Message
	Tools     []Tool
}

type Message struct {
	Role       string // "user", "assistant" or "tool"
	Text       string
	ToolCalls  []ToolCall // assistant; none means the final answer
	ToolCallID string     // tool
	IsError    bool       // tool
}

type ToolCall struct {
	ID   string
	Name string
	Args json.RawMessage
}

type ToolResult struct {
	Output  string
	IsError bool
}

// Event is one step, for the timeline.
type Event struct {
	Tenant    string
	Execution string
	Kind      string
	Data      json.RawMessage // never a secret
}
