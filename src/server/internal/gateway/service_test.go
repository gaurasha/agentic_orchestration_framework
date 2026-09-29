package gateway_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/gateway"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/gateway/deps/memory"
)

const secret = "s3cret-value"

type upstream struct{ c *http.Client }

func (u upstream) Do(ctx context.Context, r *http.Request) (*http.Response, error) {
	return u.c.Do(r.WithContext(ctx))
}

// echo is the fake external service: it reports the Authorization header
// and query it received, so a test can see what the gateway sent.
func echo(t *testing.T, seen *[]*http.Request) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.Clone(context.Background()))
		_ = json.NewEncoder(w).Encode(map[string]string{"authorization": r.Header.Get("Authorization"), "city": r.URL.Query().Get("city")})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newAPI(t *testing.T, url string, logs *bytes.Buffer) gateway.API {
	t.Helper()
	return gateway.New(gateway.Deps{
		Catalog: memory.Catalog{"acme": {
			"echo":   {Name: "echo", Kind: gateway.KindHTTP, Method: "GET", URL: url + "/echo?city={city}", Params: json.RawMessage(`{"type":"object","required":["city"],"properties":{"city":{"type":"string"}}}`), Credential: &gateway.Credential{Ref: "demo_key", Header: "Authorization", Prefix: "Bearer "}},
			"deploy": {Name: "deploy", Kind: gateway.KindHTTP, Method: "GET", URL: url + "/deploy"},
			"whoami": {Name: "whoami", Kind: gateway.KindExec, Argv: []string{"gh", "api", "user"}, Credential: &gateway.Credential{Ref: "demo_key", Env: "GH_TOKEN", Hosts: []string{"api.github.com"}}},
		}},
		Grants: memory.Grants{
			Tools:  map[string]gateway.ToolGrant{"echo": {Allowlist: map[string][]string{"city": {"London"}}}, "whoami": {}, "deploy": {NeedsApproval: true}},
			Budget: gateway.Budget{MaxCalls: 2},
		},
		Budgets:      memory.NewBudgets(),
		Journal:      memory.NewJournal(),
		Approvals:    memory.NewApprovals(),
		Credentials:  memory.Credentials{"acme": {"demo_key": secret}},
		Placeholders: placeholders,
		Sandbox:      sandbox,
		Upstream:     upstream{http.DefaultClient},
		Log:          slog.New(slog.NewTextHandler(logs, nil)),
	}, gateway.Config{Egress: "http://egress.test"})
}

var (
	placeholders = memory.NewPlaceholders()
	sandbox      = &memory.Sandbox{}
)

// 5.1: a command in the sandbox never sees the real credential: its
// environment holds a placeholder, bound to the tool's hosts, and the
// placeholder is retired when the call ends.
func TestExecGetsPlaceholderOnly(t *testing.T) {
	var logs bytes.Buffer
	api := newAPI(t, "http://unused", &logs)
	res, err := call(api, "whoami", `{"repo":"x"}`)
	if err != nil || res.Kind != gateway.KindExec || res.IsError {
		t.Fatalf("Call = %+v, %v", res, err)
	}
	if len(sandbox.Commands) != 1 {
		t.Fatalf("sandbox ran %d commands", len(sandbox.Commands))
	}
	env := sandbox.Commands[0].Env
	if !strings.HasPrefix(env["GH_TOKEN"], "fake_") || env["ARG_repo"] != "x" || env["EGRESS"] != "http://egress.test" || env["HTTPS_PROXY"] != "http://egress.test" || len(env) != 7 {
		t.Errorf("env = %v", env)
	}
	for _, v := range env {
		if strings.Contains(v, secret) {
			t.Errorf("the real credential is in the environment")
		}
	}
	if strings.Contains(res.Output, secret) || !strings.Contains(res.Output, "GH_TOKEN="+env["GH_TOKEN"]) {
		t.Errorf("output = %q", res.Output)
	}
	if len(placeholders.Issued) != 1 || placeholders.Issued[0].Hosts[0] != "api.github.com" || placeholders.Issued[0].Execution != caller.Execution {
		t.Errorf("issued = %+v", placeholders.Issued)
	}
	if placeholders.Live[env["GH_TOKEN"]] || len(placeholders.Retired) != 1 {
		t.Errorf("placeholder still live after the call: live=%v retired=%v", placeholders.Live, placeholders.Retired)
	}
	if strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), env["GH_TOKEN"]) {
		t.Errorf("log leaks: %s", logs.String())
	}
}

var caller = gateway.Caller{Tenant: "acme", Execution: "exec-1", AgentID: "a", AgentVersion: 2}

func call(api gateway.API, tool, args string) (gateway.Result, error) {
	return api.Call(context.Background(), gateway.Call{Caller: caller, Tool: tool, Args: json.RawMessage(args)})
}

func keyed(api gateway.API, key, tool, args string) (gateway.Result, error) {
	return api.Call(context.Background(), gateway.Call{Caller: caller, Tool: tool, Args: json.RawMessage(args), Key: key})
}

// 3.4: a repeat with the same key gets the saved result and the external
// service sees one request; a refused call is not saved, so a retry after
// the cause is fixed runs.
func TestRetryReplaysSavedResult(t *testing.T) {
	var seen []*http.Request
	srv := echo(t, &seen)
	var logs bytes.Buffer
	api := newAPI(t, srv.URL, &logs)

	first, err := keyed(api, "e1:1:0", "echo", `{"city":"London"}`)
	if err != nil || first.Replayed {
		t.Fatalf("first = %+v, %v", first, err)
	}
	again, err := keyed(api, "e1:1:0", "echo", `{"city":"London"}`)
	if err != nil || !again.Replayed || again.Output != first.Output {
		t.Fatalf("retry = %+v, %v; want the first result replayed", again, err)
	}
	if len(seen) != 1 {
		t.Errorf("external service saw %d requests, want 1", len(seen))
	}
	// The budget is 2: the replay did not spend it, so a new key still runs.
	if _, err := keyed(api, "e1:1:1", "echo", `{"city":"London"}`); err != nil {
		t.Errorf("second key: %v", err)
	}
	if len(seen) != 2 {
		t.Errorf("external service saw %d requests, want 2", len(seen))
	}
}

// 3.5: arguments are checked against the tool's schema before anything runs.
func TestSchemaRefusal(t *testing.T) {
	var seen []*http.Request
	srv := echo(t, &seen)
	var logs bytes.Buffer
	api := newAPI(t, srv.URL, &logs)
	for _, args := range []string{`{}`, `{"city":5}`} {
		if _, err := call(api, "echo", args); !errors.Is(err, gateway.ErrDenied) {
			t.Errorf("%s: err = %v, want ErrDenied", args, err)
		}
	}
	if len(seen) != 0 {
		t.Errorf("a refused call reached the service")
	}
}

// 4.1 and 4.2 at the gateway: a call that needs approval does not run until
// an approval for its key is on file; then the same call runs once.
func TestApprovalOnFile(t *testing.T) {
	var seen []*http.Request
	srv := echo(t, &seen)
	var logs bytes.Buffer
	api := newAPI(t, srv.URL, &logs)

	if _, err := keyed(api, "e1:2:0", "deploy", `{}`); !errors.Is(err, gateway.ErrApprovalNeeded) {
		t.Fatalf("err = %v, want ErrApprovalNeeded", err)
	}
	if len(seen) != 0 {
		t.Fatalf("an unapproved call ran")
	}
	if err := api.Approve(context.Background(), caller, "e1:2:0"); err != nil {
		t.Fatal(err)
	}
	res, err := keyed(api, "e1:2:0", "deploy", `{}`)
	if err != nil || res.IsError || res.Replayed {
		t.Fatalf("approved call = %+v, %v", res, err)
	}
	if _, err := keyed(api, "e1:3:0", "deploy", `{}`); !errors.Is(err, gateway.ErrApprovalNeeded) {
		t.Errorf("another key: err = %v, want ErrApprovalNeeded; an approval is per call", err)
	}
	if len(seen) != 1 {
		t.Errorf("service saw %d requests, want 1", len(seen))
	}
}

// 3.2: the request reaches the external service with the real credential.
// 3.3: the result and the log never contain it.
func TestCredentialAddedAndHidden(t *testing.T) {
	var seen []*http.Request
	srv := echo(t, &seen)
	var logs bytes.Buffer
	api := newAPI(t, srv.URL, &logs)

	res, err := call(api, "echo", `{"city":"London"}`)
	if err != nil || res.IsError {
		t.Fatalf("Call = %+v, %v", res, err)
	}
	if len(seen) != 1 || seen[0].Header.Get("Authorization") != "Bearer "+secret {
		t.Fatalf("external service saw %+v", seen)
	}
	if seen[0].URL.Query().Get("city") != "London" {
		t.Errorf("query = %s", seen[0].URL.RawQuery)
	}
	if strings.Contains(res.Output, secret) || !strings.Contains(res.Output, "Bearer [secret]") {
		t.Errorf("output leaks or lacks the placeholder: %s", res.Output)
	}
	if strings.Contains(logs.String(), secret) {
		t.Errorf("log leaks the credential: %s", logs.String())
	}
}

// 3.1: a call is refused, with the reason, when the version does not grant
// the tool, an argument is outside its allowlist, or the budget is spent.
func TestRefusals(t *testing.T) {
	var seen []*http.Request
	srv := echo(t, &seen)
	var logs bytes.Buffer
	api := newAPI(t, srv.URL, &logs)

	cases := []struct {
		name, tool, args string
		want             error
		reason           string
	}{
		{"argument not allowed", "echo", `{"city":"Tokyo"}`, gateway.ErrDenied, "not allowed"},
		{"tool not granted", "delete_repo", `{}`, gateway.ErrDenied, "delete_repo"},
	}
	for _, c := range cases {
		_, err := call(api, c.tool, c.args)
		if !errors.Is(err, c.want) || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: err = %v, want %v with %q", c.name, err, c.want, c.reason)
		}
	}
	if len(seen) != 0 {
		t.Fatalf("a refused call reached the external service: %d requests", len(seen))
	}

	// The budget allows two calls; refusals above did not count.
	for i := 0; i < 2; i++ {
		if _, err := call(api, "echo", `{"city":"London"}`); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
	if _, err := call(api, "echo", `{"city":"London"}`); !errors.Is(err, gateway.ErrExhausted) {
		t.Errorf("third call: err = %v, want ErrExhausted", err)
	}
	if len(seen) != 2 {
		t.Errorf("external service saw %d requests, want 2", len(seen))
	}
	if strings.Contains(logs.String(), secret) {
		t.Errorf("log leaks the credential")
	}
}

// 3.4: a call whose reply was lost after it was sent is an uncertain
// result, journaled like any other: a retry replays it and the service
// sees one request, not two.
func TestUncertainResultIsNotSentAgain(t *testing.T) {
	var seen int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen++
		w.Header().Set("Content-Length", "100") // promise more than is sent, then hang up
		_, _ = w.Write([]byte("{\"partial\":"))
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}))
	t.Cleanup(srv.Close)
	var logs bytes.Buffer
	api := newAPI(t, srv.URL, &logs)
	first, err := keyed(api, "e1:1:0", "echo", `{"city":"London"}`)
	if err != nil || !first.IsError || !first.Uncertain || !strings.Contains(first.Output, "not sent again") {
		t.Fatalf("first = %+v, %v; want an uncertain error result", first, err)
	}
	again, err := keyed(api, "e1:1:0", "echo", `{"city":"London"}`)
	if err != nil || !again.Replayed || !again.Uncertain {
		t.Fatalf("retry = %+v, %v; want the uncertain result replayed", again, err)
	}
	if seen != 1 {
		t.Errorf("service saw %d requests, want 1", seen)
	}
}
