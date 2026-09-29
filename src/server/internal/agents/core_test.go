package agents

import (
	"errors"
	"testing"
)

func TestHashIgnoresOrderAndEmptyAllowlists(t *testing.T) {
	a := Definition{Name: "a", Model: "mock", Tools: []ToolGrant{
		{Name: "x", Allowlist: map[string][]string{}},
		{Name: "y", Allowlist: map[string][]string{"city": {"London"}}},
	}}
	b := Definition{Name: "a ", Model: "mock", Tools: []ToolGrant{
		{Name: "y", Allowlist: map[string][]string{"city": {"London"}}},
		{Name: "x"},
	}}
	if hash(normalize(a)) != hash(normalize(b)) {
		t.Error("same content, different hash")
	}
	c := b
	c.Instructions = "changed"
	if hash(normalize(b)) == hash(normalize(c)) {
		t.Error("different content, same hash")
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name string
		def  Definition
		ok   bool
	}{
		{"ok", Definition{Name: "a", Model: "m", Tools: []ToolGrant{{Name: "x"}}}, true},
		{"no name", Definition{Model: "m"}, false},
		{"no model", Definition{Name: "a"}, false},
		{"negative budget", Definition{Name: "a", Model: "m", Budget: Budget{MaxCalls: -1}}, false},
		{"duplicate tool", Definition{Name: "a", Model: "m", Tools: []ToolGrant{{Name: "x"}, {Name: "x"}}}, false},
		{"unnamed tool", Definition{Name: "a", Model: "m", Tools: []ToolGrant{{}}}, false},
	}
	for _, c := range cases {
		err := validate(c.def)
		if c.ok && err != nil {
			t.Errorf("%s: unexpected %v", c.name, err)
		}
		if !c.ok && !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", c.name, err)
		}
	}
}
