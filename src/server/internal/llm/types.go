package llm

import "encoding/json"

type Request struct {
	Tenant    string
	Execution string
	Model     string
	System    string
	Messages  []Message
	Tools     []ToolSpec
}

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role
	Text       string
	ToolCalls  []ToolCall // assistant
	ToolCallID string     // tool: the call this answers
	IsError    bool       // tool
}

// ToolSpec is a tool as the model sees it.
type ToolSpec struct {
	Name        string
	Description string
	Params      json.RawMessage // JSON Schema
}

type ToolCall struct {
	ID   string
	Name string
	Args json.RawMessage
}

// Response is the model's next message; with no tool calls, its final answer.
type Response struct {
	Message Message
	Usage   Usage
}

type Usage struct {
	InputTokens  int
	OutputTokens int
}
