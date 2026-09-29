package tools

import (
	"errors"
	"testing"
)

func TestValidate(t *testing.T) {
	httpTool := Tool{Name: "echo", Kind: KindHTTP, HTTP: &HTTP{Method: "GET", URL: "http://localhost:8080/demo/echo?city={city}"}}
	execTool := Tool{Name: "whoami", Kind: KindExec, Exec: &Exec{Argv: []string{"sh", "-c", "echo $KEY"}},
		Credential: &Credential{Ref: "k", Env: "KEY", Hosts: []string{"api.example.com"}}}
	with := func(base Tool, f func(*Tool)) Tool {
		c := base
		if c.HTTP != nil {
			h := *c.HTTP
			c.HTTP = &h
		}
		if c.Exec != nil {
			e := *c.Exec
			c.Exec = &e
		}
		if c.Credential != nil {
			cr := *c.Credential
			c.Credential = &cr
		}
		f(&c)
		return c
	}
	cases := []struct {
		name string
		tool Tool
		ok   bool
	}{
		{"http, placeholder in query", httpTool, true},
		{"http, placeholder in path", with(httpTool, func(t *Tool) { t.HTTP.URL = "https://api.example.com/users/{id}" }), true},
		{"http, kind filled in", with(httpTool, func(t *Tool) { t.Kind = "" }), true},
		{"empty name", with(httpTool, func(t *Tool) { t.Name = " " }), false},
		{"bad method", with(httpTool, func(t *Tool) { t.HTTP.Method = "FETCH" }), false},
		{"placeholder in host", with(httpTool, func(t *Tool) { t.HTTP.URL = "https://{host}/x" }), false},
		{"no host", with(httpTool, func(t *Tool) { t.HTTP.URL = "/relative" }), false},
		{"userinfo", with(httpTool, func(t *Tool) { t.HTTP.URL = "https://u:p@example.com/" }), false},
		{"ftp", with(httpTool, func(t *Tool) { t.HTTP.URL = "ftp://example.com/" }), false},
		{"http credential without header", with(httpTool, func(t *Tool) { t.Credential = &Credential{Ref: "k"} }), false},
		{"http with exec too", with(httpTool, func(t *Tool) { t.Exec = &Exec{Argv: []string{"x"}} }), false},
		{"exec", execTool, true},
		{"exec without credential", with(execTool, func(t *Tool) { t.Credential = nil }), true},
		{"exec, empty argv", with(execTool, func(t *Tool) { t.Exec.Argv = nil }), false},
		{"exec, bad env name", with(execTool, func(t *Tool) { t.Credential.Env = "GH-TOKEN" }), false},
		{"exec, no hosts", with(execTool, func(t *Tool) { t.Credential.Hosts = nil }), false},
		{"exec, host with path", with(execTool, func(t *Tool) { t.Credential.Hosts = []string{"a.com/x"} }), false},
		{"unknown kind", with(execTool, func(t *Tool) { t.Kind = "shell" }), false},
	}
	for _, c := range cases {
		err := validate(normalize(c.tool))
		if c.ok && err != nil {
			t.Errorf("%s: unexpected %v", c.name, err)
		}
		if !c.ok && !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", c.name, err)
		}
	}
}
