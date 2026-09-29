// Package mock is the scripted model. It reads the user's input one line at
// a time: `tool {json args}` is a tool call, `ask <question>` asks the user
// and waits. It asks for each run of calls in one turn, then either asks
// the next question or answers with a summary, so every demo run is the
// same.
package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/executions"
)

type Model struct{}

// Segment is what the script does between two questions: some calls, then
// maybe a question.
type Segment struct {
	Calls []executions.ToolCall
	Ask   string
}

func (Model) Next(_ context.Context, p executions.Prompt) (executions.Message, error) {
	if len(p.History) == 0 {
		return executions.Message{Role: "assistant", Text: "Nothing to do."}, nil
	}
	segments := Parse(p.History[0].Text)
	answers := 0 // user messages after the input are answers to questions
	for _, m := range p.History[1:] {
		if m.Role == "user" {
			answers++
		}
	}
	last := p.History[len(p.History)-1]
	if answers >= len(segments) {
		return executions.Message{Role: "assistant", Text: summary(p.History)}, nil
	}
	seg := segments[answers]
	if last.Role == "user" && len(seg.Calls) > 0 {
		return executions.Message{Role: "assistant", ToolCalls: seg.Calls}, nil
	}
	if seg.Ask != "" {
		return executions.Message{Role: "assistant", Question: seg.Ask}, nil
	}
	return executions.Message{Role: "assistant", Text: summary(p.History)}, nil
}

// Parse splits the script at each `ask` line. A tool line without JSON
// calls the tool with no arguments; blank lines are skipped.
func Parse(input string) []Segment {
	segments := []Segment{{}}
	n := 0
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cur := &segments[len(segments)-1]
		name, rest, _ := strings.Cut(line, " ")
		rest = strings.TrimSpace(rest)
		if name == "ask" {
			cur.Ask = rest
			segments = append(segments, Segment{})
			continue
		}
		if rest == "" || !json.Valid([]byte(rest)) {
			rest = "{}"
		}
		n++
		cur.Calls = append(cur.Calls, executions.ToolCall{ID: fmt.Sprintf("call-%d", n), Name: name, Args: json.RawMessage(rest)})
	}
	return segments
}

func summary(history []executions.Message) string {
	ok, failed, answers := 0, 0, 0
	for i, m := range history {
		switch {
		case m.Role == "tool" && m.IsError:
			failed++
		case m.Role == "tool":
			ok++
		case m.Role == "user" && i > 0:
			answers++
		}
	}
	if ok+failed+answers == 0 {
		return "Nothing to do: the input named no tool calls."
	}
	s := fmt.Sprintf("Done: %d call(s) succeeded, %d refused or failed.", ok, failed)
	if answers > 0 {
		s += fmt.Sprintf(" You answered %d question(s).", answers)
	}
	return s
}
