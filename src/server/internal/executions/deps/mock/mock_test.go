package mock

import (
	"context"
	"testing"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/executions"
)

func TestParse(t *testing.T) {
	segs := Parse("echo {\"city\":\"London\"}\n\n  delete_repo  \nask Which region?\nbad {not json}\n")
	if len(segs) != 2 || len(segs[0].Calls) != 2 || segs[0].Ask != "Which region?" || len(segs[1].Calls) != 1 || segs[1].Ask != "" {
		t.Fatalf("segments = %+v", segs)
	}
	if segs[0].Calls[0].Name != "echo" || string(segs[0].Calls[0].Args) != `{"city":"London"}` || string(segs[0].Calls[1].Args) != "{}" {
		t.Errorf("calls = %+v", segs[0].Calls)
	}
}

// The script drives a whole conversation: calls, a question, more calls, a
// final answer.
func TestNext(t *testing.T) {
	ctx := context.Background()
	h := []executions.Message{{Role: "user", Text: "echo {}\nask Which region?\necho {}"}}
	next := func() executions.Message {
		m, _ := Model{}.Next(ctx, executions.Prompt{History: h})
		h = append(h, m)
		return m
	}
	if m := next(); len(m.ToolCalls) != 1 {
		t.Fatalf("turn 1 = %+v, want a call", m)
	}
	h = append(h, executions.Message{Role: "tool", ToolCallID: "call-1", Text: "ok"})
	if m := next(); m.Question != "Which region?" {
		t.Fatalf("turn 2 = %+v, want the question", m)
	}
	h = append(h, executions.Message{Role: "user", Text: "eu"})
	if m := next(); len(m.ToolCalls) != 1 {
		t.Fatalf("turn 3 = %+v, want a call", m)
	}
	h = append(h, executions.Message{Role: "tool", ToolCallID: "call-2", Text: "ok"})
	if m := next(); m.Text != "Done: 2 call(s) succeeded, 0 refused or failed. You answered 1 question(s)." {
		t.Fatalf("turn 4 = %+v, want the summary", m)
	}
}
