package timeline

import (
	"encoding/json"
	"time"
)

type Kind string

const (
	ExecutionStarted  Kind = "execution.started"
	ModelCalled       Kind = "model.called"
	ModelReplied      Kind = "model.replied"
	ToolCalled        Kind = "tool.called"
	ToolReturned      Kind = "tool.returned"
	ApprovalRequested Kind = "approval.requested"
	ApprovalDecided   Kind = "approval.decided"
	QuestionAsked     Kind = "question.asked"
	QuestionAnswered  Kind = "question.answered"
	ExecutionEnded    Kind = "execution.ended"
)

type Event struct {
	Tenant    string
	Execution string
	Kind      Kind
	Data      json.RawMessage // never a secret
}

type Entry struct {
	Event
	Seq int64 // from 1, per execution
	At  time.Time
}
