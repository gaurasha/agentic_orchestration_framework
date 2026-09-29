package executions

import (
	"encoding/json"
	"testing"
)

func TestRepeating(t *testing.T) {
	call := ToolCall{Name: "echo", Args: json.RawMessage(`{"city": "London"}`)}
	same := Step{Tool: "echo", Args: json.RawMessage(`{"city":"London"}`)}
	other := Step{Tool: "echo", Args: json.RawMessage(`{"city":"Paris"}`)}
	replay := Step{Tool: "echo", Args: json.RawMessage(`{"city":"London"}`), Replayed: true}
	cases := []struct {
		name  string
		steps []Step
		want  bool
	}{
		{"none", nil, false},
		{"twice", []Step{same, same}, false},
		{"three times", []Step{same, other, same, same}, true},
		{"replays do not count", []Step{same, replay, replay, same}, false},
		{"keys in another order are the same call", []Step{same, {Tool: "echo", Args: json.RawMessage(`{ "city" :"London" }`)}, same}, true},
	}
	nested := ToolCall{Name: "x", Args: json.RawMessage(`{"a":1,"b":{"c":[1,2],"d":"e"}}`)}
	reordered := Step{Tool: "x", Args: json.RawMessage(`{"b":{"d":"e","c":[1,2]},"a":1}`)}
	if !repeating([]Step{reordered, reordered, reordered}, nested, 3) {
		t.Errorf("reordered nested keys were treated as a different call")
	}
	if repeating([]Step{reordered, reordered, reordered}, ToolCall{Name: "x", Args: json.RawMessage(`{"a":1,"b":{"d":"e","c":[2,1]}}`)}, 3) {
		t.Errorf("a different array order was treated as the same call")
	}
	for _, c := range cases {
		if got := repeating(c.steps, call, 3); got != c.want {
			t.Errorf("%s: repeating = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestTurnsAndPending(t *testing.T) {
	h := []Message{{Role: "user"}, {Role: "assistant", ToolCalls: []ToolCall{{ID: "a"}}}, {Role: "tool"}, {Role: "assistant", ToolCalls: []ToolCall{{ID: "b"}}}}
	if turns(h) != 2 || pendingCalls(h)[0].ID != "b" || callKey("e", 2, 0) != "e:2:0" {
		t.Errorf("turns = %d, pending = %v", turns(h), pendingCalls(h))
	}
}
