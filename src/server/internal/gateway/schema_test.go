package gateway

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCheckSchema(t *testing.T) {
	params := json.RawMessage(`{"type":"object","required":["city"],"additionalProperties":false,
		"properties":{"city":{"type":"string"},"days":{"type":"integer"},"units":{"type":"string","enum":["c","f"]},"tags":{"type":"array","items":{"type":"string"}}}}`)
	cases := []struct {
		name, args, want string
	}{
		{"ok", `{"city":"London","days":3,"units":"c","tags":["a"]}`, ""},
		{"missing required", `{}`, "missing city"},
		{"wrong type", `{"city":5}`, "city must be string"},
		{"not an integer", `{"city":"x","days":1.5}`, "days must be integer"},
		{"bad enum", `{"city":"x","units":"k"}`, "not one of the allowed values"},
		{"unknown field", `{"city":"x","lang":"en"}`, "unknown field lang"},
		{"bad item", `{"city":"x","tags":[1]}`, "tags[0] must be string"},
		{"not an object", `[1]`, "arguments must be object"},
	}
	for _, c := range cases {
		err := checkSchema(params, json.RawMessage(c.args))
		if c.want == "" && err != nil {
			t.Errorf("%s: unexpected %v", c.name, err)
		}
		if c.want != "" && (!errors.Is(err, ErrDenied) || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%s: err = %v, want ErrDenied with %q", c.name, err, c.want)
		}
	}
	if err := checkSchema(nil, json.RawMessage(`{"anything":1}`)); err != nil {
		t.Errorf("no schema: %v", err)
	}
}
