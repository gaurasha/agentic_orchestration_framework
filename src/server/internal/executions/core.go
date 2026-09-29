package executions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// callKey names one call: a retry with the same key gets the saved result.
func callKey(execution string, turn, index int) string {
	return fmt.Sprintf("%s:%d:%d", execution, turn, index)
}

// questionKey names the wait for a question asked at a turn.
func questionKey(execution string, turn int) string {
	return fmt.Sprintf("%s:%d:question", execution, turn)
}

// turns counts the model's replies so far.
func turns(history []Message) int {
	n := 0
	for _, m := range history {
		if m.Role == "assistant" {
			n++
		}
	}
	return n
}

// pendingCalls are the calls of the model's last reply.
func pendingCalls(history []Message) []ToolCall {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			return history[i].ToolCalls
		}
	}
	return nil
}

// stepOf records one tool call as the model saw it.
func stepOf(index int, call ToolCall, key string, r ToolResult, at time.Time) Step {
	return Step{Index: index, Tool: call.Name, Kind: r.Kind, Args: argsOrEmpty(call.Args), Outcome: r.Outcome,
		ExitCode: r.ExitCode, Output: r.Output, Replayed: r.Replayed, At: at, Key: key}
}

// questionStep records a question and its answer.
func questionStep(index int, question, answer string, at time.Time) Step {
	return Step{Index: index, Tool: "ask", Kind: "question", Args: json.RawMessage("{}"), Outcome: OK,
		Output: "Q: " + question + "\nA: " + answer, At: at}
}

func argsOrEmpty(args json.RawMessage) json.RawMessage {
	if len(args) == 0 {
		return json.RawMessage("{}")
	}
	return args
}

// toolMessage is what the model hears back for one call.
func toolMessage(call ToolCall, r ToolResult) Message {
	text := r.Output
	switch r.Outcome {
	case Refused:
		text = "refused: " + r.Output
	case Rejected:
		text = "rejected by a human: " + r.Output
	}
	return Message{Role: "tool", ToolCallID: call.ID, Text: text, IsError: r.Outcome != OK}
}

// failed turns a tool call's transport error into a result the model can
// read. The error is the gateway's; it never carries a credential.
func failed(err error) ToolResult {
	return ToolResult{Outcome: Errored, Output: fmt.Sprintf("error: %v", err)}
}

// repeating says whether the same call has already been made max times,
// so the loop tells the model instead of running it again.
func repeating(steps []Step, call ToolCall, max int) bool {
	n := 0
	for _, s := range steps {
		if s.Tool == call.Name && !s.Replayed && sameJSON(s.Args, argsOrEmpty(call.Args)) {
			n++
		}
	}
	return n >= max
}

// sameJSON compares two JSON values by content: object keys in any order,
// whitespace ignored, so reordering keys is not a different call.
func sameJSON(a, b json.RawMessage) bool {
	var va, vb any
	if json.Unmarshal(a, &va) != nil || json.Unmarshal(b, &vb) != nil {
		return bytes.Equal(bytes.TrimSpace(a), bytes.TrimSpace(b))
	}
	ca, _ := json.Marshal(va) // encoding/json writes map keys sorted
	cb, _ := json.Marshal(vb)
	return bytes.Equal(ca, cb)
}
