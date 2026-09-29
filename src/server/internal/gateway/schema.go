package gateway

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// A small JSON Schema check, enough for tool arguments: an object with
// typed properties, required names, and additionalProperties.

type schema struct {
	Type                 string            `json:"type"`
	Properties           map[string]schema `json:"properties"`
	Required             []string          `json:"required"`
	AdditionalProperties *bool             `json:"additionalProperties"`
	Enum                 []json.RawMessage `json:"enum"`
	Items                *schema           `json:"items"`
}

// checkSchema refuses arguments that do not fit the tool's schema. An
// empty schema accepts anything.
func checkSchema(params, args json.RawMessage) error {
	if len(params) == 0 || string(params) == "null" {
		return nil
	}
	var s schema
	if err := json.Unmarshal(params, &s); err != nil {
		return fmt.Errorf("%w: the tool's argument schema is not valid: %v", ErrDenied, err)
	}
	var v any
	if len(args) == 0 || string(args) == "null" {
		v = map[string]any{}
	} else if err := json.Unmarshal(args, &v); err != nil {
		return fmt.Errorf("%w: arguments must be JSON", ErrDenied)
	}
	if err := check(s, v, "arguments"); err != nil {
		return fmt.Errorf("%w: %v", ErrDenied, err)
	}
	return nil
}

func check(s schema, v any, at string) error {
	if s.Type != "" && !isType(s.Type, v) {
		return fmt.Errorf("%s must be %s", at, s.Type)
	}
	if len(s.Enum) > 0 {
		got, _ := json.Marshal(v)
		ok := false
		for _, e := range s.Enum {
			if string(e) == string(got) {
				ok = true
			}
		}
		if !ok {
			return fmt.Errorf("%s is not one of the allowed values", at)
		}
	}
	switch x := v.(type) {
	case map[string]any:
		for _, r := range s.Required {
			if _, ok := x[r]; !ok {
				return fmt.Errorf("%s is missing %s", at, r)
			}
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			ps, known := s.Properties[k]
			if !known {
				if s.AdditionalProperties != nil && !*s.AdditionalProperties {
					return fmt.Errorf("%s has an unknown field %s", at, k)
				}
				continue
			}
			if err := check(ps, x[k], k); err != nil {
				return err
			}
		}
	case []any:
		if s.Items != nil {
			for i, item := range x {
				if err := check(*s.Items, item, fmt.Sprintf("%s[%d]", at, i)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func isType(t string, v any) bool {
	switch t {
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "number":
		_, ok := v.(float64)
		return ok
	case "integer":
		f, ok := v.(float64)
		return ok && f == math.Trunc(f)
	case "null":
		return v == nil
	}
	return true
}
