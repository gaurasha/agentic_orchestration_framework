package gateway

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestCheckGrant(t *testing.T) {
	g := Grant{Tools: map[string]ToolGrant{
		"echo":   {Allowlist: map[string][]string{"city": {"London", "Paris"}}},
		"any":    {},
		"gated":  {NeedsApproval: true},
		"nested": {Allowlist: map[string][]string{"n": {"1"}}},
	}}
	cases := []struct {
		name string
		tool string
		args string
		want string // "" for allowed, else a substring of the reason
	}{
		{"allowed", "echo", `{"city":"London"}`, ""},
		{"no args with allowlist", "echo", `{}`, ""},
		{"value not allowed", "echo", `{"city":"Tokyo"}`, `city="Tokyo" is not allowed`},
		{"argument not listed", "echo", `{"city":"London","lang":"en"}`, "argument lang is not allowed"},
		{"tool not granted", "delete_repo", `{}`, "not granted"},
		{"empty allowlist allows any", "any", `{"x":"y"}`, ""},
		{"number compares as text", "nested", `{"n":1}`, ""},
		{"null args", "echo", `null`, ""},
	}
	for _, c := range cases {
		args, err := argsOf(json.RawMessage(c.args))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		_, err = checkGrant(g, c.tool, args)
		if c.want == "" && err != nil {
			t.Errorf("%s: unexpected %v", c.name, err)
		}
		if c.want != "" && (!errors.Is(err, ErrDenied) || !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%s: err = %v, want ErrDenied with %q", c.name, err, c.want)
		}
	}
	if _, err := argsOf(json.RawMessage(`[1]`)); !errors.Is(err, ErrDenied) {
		t.Errorf("array args: err = %v, want ErrDenied", err)
	}
}

func TestBuildURLEscapesByPosition(t *testing.T) {
	u, err := buildURL("https://api.example/items/{name}/sub?q={q}&fixed=1", map[string]string{"name": "hello world/x", "q": "a b&c"})
	if err != nil {
		t.Fatal(err)
	}
	if u.EscapedPath() != "/items/hello%20world%2Fx/sub" || u.RawQuery != "q=a+b%26c&fixed=1" || u.Host != "api.example" {
		t.Errorf("got path %q query %q host %q", u.EscapedPath(), u.RawQuery, u.Host)
	}
}

func TestBuildURL(t *testing.T) {
	u, err := buildURL("http://h:1/x/{id}?city={city}", map[string]string{"id": "a/b", "city": "New York"})
	if err != nil || u.String() != "http://h:1/x/a%2Fb?city=New+York" {
		t.Errorf("got %v, %v", u, err)
	}
	if _, err := buildURL("http://h/{id}", nil); !errors.Is(err, ErrDenied) {
		t.Errorf("missing arg: err = %v, want ErrDenied", err)
	}
}

func TestCommandEnv(t *testing.T) {
	env, err := commandEnv(map[string]string{"city": "London"}, "http://egress", "GH_TOKEN", "fake_x")
	if err != nil || env["ARG_city"] != "London" || env["EGRESS"] != "http://egress" || env["HTTPS_PROXY"] != "http://egress" || env["http_proxy"] != "http://egress" || env["GH_TOKEN"] != "fake_x" || len(env) != 7 {
		t.Errorf("commandEnv = %v, %v", env, err)
	}
	if _, err := commandEnv(map[string]string{"bad-name": "x"}, "", "", ""); !errors.Is(err, ErrDenied) {
		t.Errorf("bad name: err = %v, want ErrDenied", err)
	}
}

func TestRedact(t *testing.T) {
	got := redact(`{"auth":"Bearer s3cret","again":"s3cret"}`, "s3cret", 1000)
	if strings.Contains(got, "s3cret") || !strings.Contains(got, "[secret]") {
		t.Errorf("got %q", got)
	}
	// 3.3: the encodings a reply commonly uses hide it too.
	secret := `p@ss"w\rd&x`
	basic := base64.StdEncoding.EncodeToString([]byte("user:" + secret))
	body, _ := json.Marshal(map[string]string{"echoed": secret, "url": url.QueryEscape(secret), "basic": "Basic " + basic, "short": base64.StdEncoding.EncodeToString([]byte("u:" + secret))})
	got = redact(string(body), secret, 10000)
	for name, form := range map[string]string{"plain": secret, "json": `p@ss\"w\\rd&x`, "url": url.QueryEscape(secret), "basic": basic} {
		if strings.Contains(got, form) {
			t.Errorf("%s form of the credential survived: %s", name, got)
		}
	}
	if !strings.Contains(got, "[secret]") {
		t.Errorf("nothing was masked: %s", got)
	}
	var back map[string]string
	if err := json.Unmarshal([]byte(got), &back); err != nil {
		t.Errorf("redaction broke the JSON: %v in %s", err, got)
	}
	if raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(back["basic"], "Basic ")); string(raw) != "user:[secret]" {
		t.Errorf("basic credential decodes to %q, want user:[secret]", raw)
	}
	if got := redact(strings.Repeat("a", 10), "", 4); got != "aaaa…[truncated]" {
		t.Errorf("cap: got %q", got)
	}
}
