package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

const secret = "demo-s3cret-value"

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

// client drives the API as one tenant and keeps every response body, so a
// test can check that none of them leaks.
type client struct {
	t      *testing.T
	srv    *httptest.Server
	token  string
	bodies *[]string
}

type servers struct {
	app                 *app
	api, tlsAPI, egress *httptest.Server
	logs                *bytes.Buffer
	bodies              *[]string
}

func newServers(t *testing.T) servers {
	t.Helper()
	var logs bytes.Buffer
	eg := httptest.NewUnstartedServer(nil)
	// The API server again over TLS, standing in for an external https
	// service such as api.github.com; egress trusts its certificate.
	var app *app
	tlsAPI := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { app.handler().ServeHTTP(w, r) }))
	tlsAPI.StartTLS()
	roots := x509.NewCertPool()
	roots.AddCert(tlsAPI.Certificate())
	app, err := newApp(appConfig{
		SigningKey: "test-signing-key", EgressURL: "http://" + eg.Listener.Addr().String(), CAFile: filepath.Join(t.TempDir(), "ca.pem"),
		Sandbox: "local", AllowHTTPEgress: true, UpstreamRoots: roots,
	}, slog.New(slog.NewTextHandler(&logs, nil)), fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	eg.Config.Handler = app.egressHandler()
	eg.Start()
	api := httptest.NewServer(app.handler())
	t.Cleanup(api.Close)
	t.Cleanup(tlsAPI.Close)
	t.Cleanup(eg.Close)
	return servers{app: app, api: api, tlsAPI: tlsAPI, egress: eg, logs: &logs, bodies: &[]string{}}
}

func signIn(t *testing.T, s servers, tenant string) client {
	t.Helper()
	c := client{t: t, srv: s.api, bodies: s.bodies}
	var out struct{ Token string }
	c.do("POST", "/v1/token", map[string]string{"tenant": tenant}, http.StatusOK, &out)
	c.token = out.Token
	return c
}

func (c client) do(method, path string, in any, want int, out any) {
	c.t.Helper()
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.srv.URL+path, body)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	text, _ := io.ReadAll(res.Body)
	*c.bodies = append(*c.bodies, string(text))
	if res.StatusCode != want {
		c.t.Fatalf("%s %s = %d %s, want %d", method, path, res.StatusCode, text, want)
	}
	if out != nil {
		reflect.ValueOf(out).Elem().Set(reflect.Zero(reflect.TypeOf(out).Elem())) // a fresh decode, not a merge
		if err := json.Unmarshal(text, out); err != nil {
			c.t.Fatalf("%s %s: %v in %s", method, path, err, text)
		}
	}
}

type execution struct {
	ID          string
	Status      string
	Description string
	Wait        *struct{ Key, Kind, Tool, Question string }
	Output      string
	Error       string
	Steps       []struct {
		Index                 int
		Outcome, Output, Kind string
		ExitCode              *int
		Replayed, Approved    bool
	}
}

// setUp stores a credential, registers the echo tool against the server's
// own /demo/echo, and creates an agent allowed to call it for London only.
func setUp(c client) (agentID string) {
	c.do("PUT", "/v1/credentials/demo_key", map[string]string{"value": secret}, http.StatusNoContent, nil)
	c.do("PUT", "/v1/tools/echo", map[string]any{
		"description": "echoes", "kind": "http",
		"http":       map[string]string{"method": "GET", "url": c.srv.URL + "/demo/echo?city={city}"},
		"credential": map[string]string{"ref": "demo_key", "header": "Authorization", "prefix": "Bearer "},
	}, http.StatusOK, nil)
	var agent struct{ ID string }
	c.do("POST", "/v1/agents", map[string]any{
		"name": "demo", "model": "mock", "budget": map[string]int{"maxCalls": 2},
		"tools": []map[string]any{{"name": "echo", "allowlist": map[string][]string{"city": {"London"}}}},
	}, http.StatusCreated, &agent)
	return agent.ID
}

func assertNoLeak(t *testing.T, s servers) {
	t.Helper()
	for _, b := range *s.bodies {
		if strings.Contains(b, secret) {
			t.Errorf("an API response leaks the credential: %s", b)
		}
	}
	if strings.Contains(s.logs.String(), secret) {
		t.Errorf("a log line leaks the credential:\n%s", s.logs.String())
	}
}

// 3.2 and 3.3 end to end: the demo service sees the real credential; no API
// response, execution step or log line does.
func TestCredentialReachesServiceButNothingElse(t *testing.T) {
	s := newServers(t)
	c := signIn(t, s, "acme")
	agentID := setUp(c)

	var e execution
	input := "echo {\"city\":\"London\"}\necho {\"city\":\"Tokyo\"}\ndelete_repo {}\necho {\"city\":\"London\"}\necho {\"city\":\"London\"}"
	c.do("POST", "/v1/executions", map[string]string{"agentId": agentID, "input": input}, http.StatusCreated, &e)
	if e.Status != "succeeded" || len(e.Steps) != 5 {
		t.Fatalf("execution = %+v", e)
	}
	want := []struct{ outcome, output string }{
		{"ok", `"Authorization":"Bearer [secret]"`},
		{"refused", `city="Tokyo" is not allowed`},
		{"refused", "delete_repo is not in the registry"},
		{"ok", "Bearer [secret]"},
		{"refused", "budget exhausted"},
	}
	for i, w := range want {
		if e.Steps[i].Outcome != w.outcome || !strings.Contains(e.Steps[i].Output, w.output) {
			t.Errorf("step %d = %+v, want %s with %q", i+1, e.Steps[i], w.outcome, w.output)
		}
	}
	c.do("GET", "/v1/executions", nil, http.StatusOK, nil)
	c.do("GET", "/v1/credentials", nil, http.StatusOK, nil)
	c.do("GET", "/v1/tools", nil, http.StatusOK, nil)
	assertNoLeak(t, s)
	if !strings.Contains(s.logs.String(), "tool call started") || !strings.Contains(s.logs.String(), "tool call finished") {
		t.Errorf("the gateway did not log before and after the call:\n%s", s.logs.String())
	}
}

var placeholderPattern = regexp.MustCompile(`fake_[0-9a-f]{32}`)

// 5.1, 5.2 and 5.3 end to end: a command in the local sandbox sees only a
// placeholder; its request through egress reaches the external service
// with the real credential; the placeholder is dead once the call ends.
func TestSandboxedCommandUsesPlaceholder(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl is not installed")
	}
	s := newServers(t)
	c := signIn(t, s, "acme")
	c.do("PUT", "/v1/credentials/gh", map[string]string{"value": secret}, http.StatusNoContent, nil)
	host := strings.TrimPrefix(s.api.URL, "http://")
	// The command asks egress to reach the API server's /demo/echo, which
	// stands in for the external service, and prints what it got back.
	c.do("PUT", "/v1/tools/whoami", map[string]any{
		"description": "who am I", "kind": "exec",
		"exec":       map[string]any{"argv": []string{"sh", "-c", `echo "env=$GH_TOKEN"; curl -sS -H "Authorization: Bearer $GH_TOKEN" "$EGRESS/http/` + host + `/demo/echo?who=$ARG_who"`}},
		"credential": map[string]any{"ref": "gh", "env": "GH_TOKEN", "hosts": []string{host}},
	}, http.StatusOK, nil)
	c.do("PUT", "/v1/tools/elsewhere", map[string]any{
		"description": "tries another host", "kind": "exec",
		"exec":       map[string]any{"argv": []string{"sh", "-c", `curl -sS -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $GH_TOKEN" "$EGRESS/http/other.example/x"`}},
		"credential": map[string]any{"ref": "gh", "env": "GH_TOKEN", "hosts": []string{host}},
	}, http.StatusOK, nil)
	var agent struct{ ID string }
	c.do("POST", "/v1/agents", map[string]any{
		"name": "cli", "model": "mock", "tools": []map[string]any{{"name": "whoami"}, {"name": "elsewhere"}},
	}, http.StatusCreated, &agent)

	var e execution
	c.do("POST", "/v1/executions", map[string]string{"agentId": agent.ID, "input": "whoami {\"who\":\"me\"}\nelsewhere {}"}, http.StatusCreated, &e)
	if e.Status != "succeeded" || len(e.Steps) != 2 {
		t.Fatalf("execution = %+v", e)
	}
	step := e.Steps[0]
	if step.Outcome != "ok" || step.Kind != "exec" || step.ExitCode == nil || *step.ExitCode != 0 {
		t.Fatalf("whoami step = %+v", step)
	}
	ph := placeholderPattern.FindString(step.Output)
	if ph == "" || !strings.Contains(step.Output, "env="+ph) {
		t.Fatalf("the command did not print a placeholder from its environment:\n%s", step.Output)
	}
	// 5.2: the external service saw the real credential (it echoes the
	// header; egress put the placeholder back), and the query carried the
	// argument from the environment.
	if !strings.Contains(step.Output, `"Authorization":"Bearer `+ph+`"`) || !strings.Contains(step.Output, `"who":["me"]`) {
		t.Errorf("the service did not see the swapped credential:\n%s", step.Output)
	}
	if strings.Contains(step.Output, secret) {
		t.Errorf("the real credential reached the sandbox:\n%s", step.Output)
	}
	// 5.2: the same placeholder is refused for any other host.
	if e.Steps[1].Outcome != "ok" || !strings.Contains(e.Steps[1].Output, "403") {
		t.Errorf("other host: step = %+v, want a 403 from egress", e.Steps[1])
	}
	// 5.3: the placeholder is dead after the call.
	req, _ := http.NewRequest("GET", s.egress.URL+"/http/"+host+"/demo/echo", nil)
	req.Header.Set("Authorization", "Bearer "+ph)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("retired placeholder: egress = %d, want 401", res.StatusCode)
	}
	assertNoLeak(t, s)
	if strings.Contains(s.logs.String(), ph) {
		t.Errorf("a log line names the placeholder:\n%s", s.logs.String())
	}
}

// 4.1–4.4 end to end: a call that needs approval pauses the run; approve
// runs it and the run finishes; reject sends the reason to the model; a
// question waits for its answer; a waiting run can be cancelled.
func TestApprovalsQuestionsAndCancel(t *testing.T) {
	s := newServers(t)
	c := signIn(t, s, "acme")
	agentID := setUp(c)
	c.do("PUT", "/v1/tools/deploy", map[string]any{
		"description": "deploys", "kind": "http", "http": map[string]string{"method": "GET", "url": c.srv.URL + "/demo/echo?env={env}"},
	}, http.StatusOK, nil)
	var agent struct{ ID string }
	c.do("PUT", "/v1/agents/"+agentID, map[string]any{
		"name": "demo", "model": "mock", "budget": map[string]int{"maxCalls": 10},
		"tools": []map[string]any{
			{"name": "echo", "allowlist": map[string][]string{"city": {"London"}}},
			{"name": "deploy", "needsApproval": true},
		},
	}, http.StatusOK, &agent)

	// Approve.
	var e execution
	c.do("POST", "/v1/executions", map[string]string{"agentId": agentID, "input": "deploy {\"env\":\"prod\"}\necho {\"city\":\"London\"}"}, http.StatusCreated, &e)
	if e.Status != "waiting" || e.Wait == nil || e.Wait.Kind != "approval" || e.Wait.Tool != "deploy" || len(e.Steps) != 0 {
		t.Fatalf("after start: %+v", e)
	}
	c.do("POST", "/v1/executions/"+e.ID+"/answer", map[string]string{"text": "yes", "key": e.Wait.Key}, http.StatusConflict, nil)
	c.do("POST", "/v1/executions/"+e.ID+"/decide", map[string]any{"approve": true, "key": "stale"}, http.StatusConflict, nil)
	c.do("POST", "/v1/executions/"+e.ID+"/decide", map[string]any{"approve": true, "key": e.Wait.Key}, http.StatusOK, &e)
	if e.Status != "succeeded" || len(e.Steps) != 2 || !e.Steps[0].Approved || e.Steps[0].Outcome != "ok" || e.Steps[1].Outcome != "ok" {
		t.Fatalf("after approve: %+v", e)
	}
	if !strings.Contains(e.Steps[0].Output, `"env":["prod"]`) {
		t.Errorf("the approved call did not run: %+v", e.Steps[0])
	}

	// Reject.
	c.do("POST", "/v1/executions", map[string]string{"agentId": agentID, "input": "deploy {}\necho {\"city\":\"London\"}"}, http.StatusCreated, &e)
	c.do("POST", "/v1/executions/"+e.ID+"/decide", map[string]any{"approve": false, "reason": "not on a Friday", "key": e.Wait.Key}, http.StatusOK, &e)
	if e.Status != "succeeded" || len(e.Steps) != 2 || e.Steps[0].Outcome != "rejected" || e.Steps[0].Output != "not on a Friday" || e.Steps[1].Outcome != "ok" {
		t.Fatalf("after reject: %+v", e)
	}
	if e.Output != "Done: 1 call(s) succeeded, 1 refused or failed." {
		t.Errorf("the model did not hear the rejection: %q", e.Output)
	}

	// Question and answer.
	c.do("POST", "/v1/executions", map[string]string{"agentId": agentID, "input": "ask Which city?\necho {\"city\":\"London\"}"}, http.StatusCreated, &e)
	if e.Status != "waiting" || e.Wait == nil || e.Wait.Kind != "question" || e.Wait.Question != "Which city?" {
		t.Fatalf("after ask: %+v", e)
	}
	c.do("POST", "/v1/executions/"+e.ID+"/decide", map[string]any{"approve": true, "key": e.Wait.Key}, http.StatusConflict, nil)
	c.do("POST", "/v1/executions/"+e.ID+"/answer", map[string]string{"text": "London", "key": e.Wait.Key}, http.StatusOK, &e)
	if e.Status != "succeeded" || len(e.Steps) != 2 || e.Steps[0].Kind != "question" || e.Steps[1].Outcome != "ok" {
		t.Fatalf("after answer: %+v", e)
	}

	// Cancel.
	c.do("POST", "/v1/executions", map[string]string{"agentId": agentID, "input": "deploy {}"}, http.StatusCreated, &e)
	c.do("POST", "/v1/executions/"+e.ID+"/cancel", map[string]string{"reason": "changed my mind"}, http.StatusOK, &e)
	if e.Status != "cancelled" || e.Wait != nil || len(e.Steps) != 0 || e.Error != "cancelled: changed my mind" {
		t.Fatalf("after cancel: %+v", e)
	}
	c.do("POST", "/v1/executions/"+e.ID+"/decide", map[string]any{"approve": true, "key": "gone"}, http.StatusConflict, nil)
	assertNoLeak(t, s)
}

// 3.4 end to end: a retried step is served from the journal; the demo
// service's request counter shows nothing ran twice.
func TestRetryIsReplayed(t *testing.T) {
	s := newServers(t)
	c := signIn(t, s, "acme")
	agentID := setUp(c)
	var e execution
	c.do("POST", "/v1/executions", map[string]string{"agentId": agentID, "input": "echo {\"city\":\"London\"}"}, http.StatusCreated, &e)
	first := e.Steps[0].Output
	c.do("POST", "/v1/executions/"+e.ID+"/steps/1/retry", nil, http.StatusOK, &e)
	if len(e.Steps) != 2 || !e.Steps[1].Replayed || e.Steps[1].Output != first || !strings.Contains(first, `"request":1`) {
		t.Fatalf("retry = %+v", e.Steps)
	}
	c.do("POST", "/v1/executions/"+e.ID+"/steps/9/retry", nil, http.StatusBadRequest, nil)
}

// 5.4: files a command writes are there for the run's next command.
func TestSandboxKeepsFilesAcrossCommands(t *testing.T) {
	s := newServers(t)
	c := signIn(t, s, "acme")
	c.do("PUT", "/v1/tools/write", map[string]any{"description": "writes", "kind": "exec",
		"exec": map[string]any{"argv": []string{"sh", "-c", `echo "$ARG_text" > note.txt`}}}, http.StatusOK, nil)
	c.do("PUT", "/v1/tools/read", map[string]any{"description": "reads", "kind": "exec",
		"exec": map[string]any{"argv": []string{"cat", "note.txt"}}}, http.StatusOK, nil)
	var agent struct{ ID string }
	c.do("POST", "/v1/agents", map[string]any{"name": "files", "model": "mock", "tools": []map[string]any{{"name": "write"}, {"name": "read"}}}, http.StatusCreated, &agent)
	var e execution
	c.do("POST", "/v1/executions", map[string]string{"agentId": agent.ID, "input": "write {\"text\":\"hello\"}\nread {}"}, http.StatusCreated, &e)
	if e.Status != "succeeded" || len(e.Steps) != 2 || e.Steps[1].Outcome != "ok" || !strings.HasPrefix(e.Steps[1].Output, "hello") {
		t.Fatalf("execution = %+v", e)
	}
	first := e.ID
	// A different run has its own sandbox and does not see the file.
	c.do("POST", "/v1/executions", map[string]string{"agentId": agent.ID, "input": "read {}"}, http.StatusCreated, &e)
	if e.Steps[0].Outcome != "error" {
		t.Errorf("another run saw the file: %+v", e.Steps[0])
	}
	// Both runs ended, so both sandboxes are gone.
	for _, id := range []string{first, e.ID} {
		if dirs, _ := filepath.Glob(filepath.Join(os.TempDir(), "sandbox-acme-"+id+"-*")); len(dirs) != 0 {
			t.Errorf("sandbox left behind: %v", dirs)
		}
	}
}

// 5.2 with a Basic credential, as git and curl -u send it: the placeholder
// is found inside the encoding, swapped, and the reply hides the real value.
func TestBasicCredentialThroughEgress(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl is not installed")
	}
	s := newServers(t)
	c := signIn(t, s, "acme")
	c.do("PUT", "/v1/credentials/gh", map[string]string{"value": secret}, http.StatusNoContent, nil)
	host := strings.TrimPrefix(s.api.URL, "http://")
	c.do("PUT", "/v1/tools/basic", map[string]any{
		"description": "basic auth", "kind": "exec",
		"exec":       map[string]any{"argv": []string{"sh", "-c", `echo "env: $GH_TOKEN"; curl -sS -u "x-access-token:$GH_TOKEN" "$EGRESS/http/` + host + `/demo/echo"`}},
		"credential": map[string]any{"ref": "gh", "env": "GH_TOKEN", "hosts": []string{host}},
	}, http.StatusOK, nil)
	var agent struct{ ID string }
	c.do("POST", "/v1/agents", map[string]any{"name": "basic", "model": "mock", "tools": []map[string]any{{"name": "basic"}}}, http.StatusCreated, &agent)
	var e execution
	c.do("POST", "/v1/executions", map[string]string{"agentId": agent.ID, "input": "basic {}"}, http.StatusCreated, &e)
	if e.Status != "succeeded" || e.Steps[0].Outcome != "ok" {
		t.Fatalf("execution = %+v", e)
	}
	// The service reports the header it received; egress hid the real value,
	// inside the Basic encoding too, so the sandbox saw only the placeholder.
	ph := placeholderPattern.FindString(e.Steps[0].Output)
	want := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + ph))
	leaked := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + secret))
	if ph == "" || !strings.Contains(e.Steps[0].Output, `"Authorization":"Basic `+want) || strings.Contains(e.Steps[0].Output, leaked) {
		t.Errorf("output = %s", e.Steps[0].Output)
	}
	// An unauthenticated probe gets a challenge, so git retries with its credentials.
	res, err := http.Get(s.egress.URL + "/http/" + host + "/demo/echo")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized || res.Header.Get("WWW-Authenticate") == "" {
		t.Errorf("probe = %d %v, want 401 with a challenge", res.StatusCode, res.Header)
	}
	assertNoLeak(t, s)
}

// 5.2 with a real CLI: git in the sandbox holds only the placeholder, and
// the fake git server lists its branch only to the real stored value, so
// a branch in git's output proves egress swapped the credential.
func TestGitThroughEgress(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	s := newServers(t)
	c := signIn(t, s, "acme")
	c.do("PUT", "/v1/credentials/gh", map[string]string{"value": secret}, http.StatusNoContent, nil)
	host := strings.TrimPrefix(s.api.URL, "http://")
	c.do("PUT", "/v1/tools/refs", map[string]any{
		"description": "ls-remote", "kind": "exec",
		"exec": map[string]any{"argv": []string{"sh", "-c",
			`echo "env: $GH_TOKEN"; git -c credential.helper= ls-remote --heads "http://x-access-token:$GH_TOKEN@${EGRESS#http://}/http/` + host + `/demo/git/acme/gh/$ARG_repo.git"`}},
		"credential": map[string]any{"ref": "gh", "env": "GH_TOKEN", "hosts": []string{host}},
	}, http.StatusOK, nil)
	var agent struct{ ID string }
	c.do("POST", "/v1/agents", map[string]any{"name": "git", "model": "mock", "tools": []map[string]any{{"name": "refs", "allowlist": map[string][]string{"repo": {"acme/app"}}}}}, http.StatusCreated, &agent)
	var e execution
	c.do("POST", "/v1/executions", map[string]string{"agentId": agent.ID, "input": "refs {\"repo\":\"acme/app\"}\nrefs {\"repo\":\"evil/app\"}"}, http.StatusCreated, &e)
	if e.Status != "succeeded" || len(e.Steps) != 2 {
		t.Fatalf("execution = %+v", e)
	}
	step := e.Steps[0]
	ph := placeholderPattern.FindString(step.Output)
	if step.Outcome != "ok" || ph == "" || !strings.Contains(step.Output, "refs/heads/real-credential-received") {
		t.Fatalf("git step = %+v", step)
	}
	if e.Steps[1].Outcome != "refused" {
		t.Errorf("other repo = %+v, want refused by the allowlist", e.Steps[1])
	}
	// The placeholder itself does not open the git server: only egress's swap did.
	req, _ := http.NewRequest("GET", s.api.URL+"/demo/git/acme/gh/acme/app.git/info/refs", nil)
	req.SetBasicAuth("x-access-token", ph)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("git server accepted the placeholder: %d", res.StatusCode)
	}
	assertNoLeak(t, s)
}

// 8.1: tenant A cannot list, read, run or use tenant B's agents, executions,
// tools or credentials; a request without a valid token is refused.
func TestTenantIsolation(t *testing.T) {
	s := newServers(t)
	acme := signIn(t, s, "acme")
	agentID := setUp(acme)
	var e struct{ ID string }
	acme.do("POST", "/v1/executions", map[string]string{"agentId": agentID, "input": "echo {\"city\":\"London\"}"}, http.StatusCreated, &e)

	globex := signIn(t, s, "globex")
	for _, path := range []string{"/v1/agents", "/v1/tools", "/v1/credentials", "/v1/executions"} {
		var list []any
		globex.do("GET", path, nil, http.StatusOK, &list)
		if len(list) != 0 {
			t.Errorf("globex lists acme's %s: %v", path, list)
		}
	}
	globex.do("GET", "/v1/agents/"+agentID+"/versions", nil, http.StatusNotFound, nil)
	globex.do("PUT", "/v1/agents/"+agentID, map[string]any{"name": "x", "model": "mock"}, http.StatusNotFound, nil)
	globex.do("GET", "/v1/executions/"+e.ID, nil, http.StatusNotFound, nil)
	globex.do("POST", "/v1/executions", map[string]string{"agentId": agentID, "input": ""}, http.StatusNotFound, nil)
	// globex cannot grant acme's tool, nor name acme's credential in a tool.
	globex.do("POST", "/v1/agents", map[string]any{"name": "x", "model": "mock", "tools": []map[string]any{{"name": "echo"}}}, http.StatusBadRequest, nil)

	none := client{t: t, srv: s.api, bodies: s.bodies}
	none.do("GET", "/v1/agents", nil, http.StatusUnauthorized, nil)
	forged := client{t: t, srv: s.api, bodies: s.bodies, token: acme.token[:len(acme.token)-3] + "xyz"}
	forged.do("GET", "/v1/agents", nil, http.StatusUnauthorized, nil)
}

// The startup seed loads every demo without an error, and leaves each run
// where the README says: two finished, two waiting on a human.
func TestSeed(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl is not installed")
	}
	s := newServers(t)
	if err := s.app.seed(t.Context(), seedOptions{Tenant: "acme", APIHost: strings.TrimPrefix(s.api.URL, "http://"), RunCommands: true, GitHubToken: secret}); err != nil {
		t.Fatal(err)
	}
	c := signIn(t, s, "acme")
	var runs []execution
	c.do("GET", "/v1/executions", nil, http.StatusOK, &runs)
	got := map[string]int{}
	for _, e := range runs {
		status := e.Status
		if e.Wait != nil {
			status += " " + e.Wait.Kind
		}
		got[status]++
		if e.Description == "" {
			t.Errorf("seeded run %s has no description saying what to expect", e.ID)
		}
		for _, st := range e.Steps {
			if st.Outcome == "error" {
				t.Errorf("run %s step %d failed: %s", e.ID, st.Index, st.Output)
			}
		}
	}
	want := map[string]int{"succeeded": 7, "waiting approval": 1, "waiting question": 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("seeded runs = %v, want %v", got, want)
	}
	var seeded []struct{ Name string }
	c.do("GET", "/v1/agents", nil, http.StatusOK, &seeded)
	if len(seeded) != 10 {
		t.Errorf("seeded agents = %+v, want the ten demos", seeded)
	}
	c.do("GET", "/v1/credentials", nil, http.StatusOK, nil)
	assertNoLeak(t, s) // the seeded GitHub token stays out of every body and log line
}

// 5.5 end to end: a CLI that fixes its host, as gh does. curl in the local
// sandbox calls an https URL as written, with the HTTP proxy the gateway
// set and the platform CA the runtime named, so it sends a CONNECT to
// egress and trusts the certificate egress answers with. Egress swaps the
// placeholder inside the TLS connection and hides the real value in the
// reply; another host is refused; the placeholder is dead after the call;
// a client that does not trust the platform CA gets nothing.
func TestCLIThroughProxy(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl is not installed")
	}
	s := newServers(t)
	c := signIn(t, s, "acme")
	c.do("PUT", "/v1/credentials/gh", map[string]string{"value": secret}, http.StatusNoContent, nil)
	host := strings.TrimPrefix(s.tlsAPI.URL, "https://")
	c.do("PUT", "/v1/tools/api", map[string]any{
		"description": "calls the https service as written", "kind": "exec",
		"exec":       map[string]any{"argv": []string{"sh", "-c", `echo "env: $GH_TOKEN"; curl -sS -H "Authorization: token $GH_TOKEN" "https://` + host + `/demo/echo?who=$ARG_who"`}},
		"credential": map[string]any{"ref": "gh", "env": "GH_TOKEN", "hosts": []string{host}},
	}, http.StatusOK, nil)
	c.do("PUT", "/v1/tools/elsewhere", map[string]any{
		"description": "tries another host", "kind": "exec",
		"exec":       map[string]any{"argv": []string{"sh", "-c", `curl -sS -o /dev/null -w "%{http_code}" -H "Authorization: token $GH_TOKEN" https://other.example/x`}},
		"credential": map[string]any{"ref": "gh", "env": "GH_TOKEN", "hosts": []string{host}},
	}, http.StatusOK, nil)
	var agent struct{ ID string }
	c.do("POST", "/v1/agents", map[string]any{"name": "cli", "model": "mock", "tools": []map[string]any{{"name": "api"}, {"name": "elsewhere"}}}, http.StatusCreated, &agent)

	var e execution
	c.do("POST", "/v1/executions", map[string]string{"agentId": agent.ID, "input": "api {\"who\":\"me\"}\nelsewhere {}"}, http.StatusCreated, &e)
	if e.Status != "succeeded" || len(e.Steps) != 2 {
		t.Fatalf("execution = %+v", e)
	}
	step := e.Steps[0]
	ph := placeholderPattern.FindString(step.Output)
	if step.Outcome != "ok" || ph == "" || !strings.HasPrefix(step.Output, "env: "+ph) {
		t.Fatalf("api step = %+v", step)
	}
	if !strings.Contains(step.Output, `"Authorization":"token `+ph+`"`) || !strings.Contains(step.Output, `"who":["me"]`) {
		t.Errorf("the service did not see the swapped credential through the tunnel:\n%s", step.Output)
	}
	if strings.Contains(step.Output, secret) {
		t.Errorf("the real credential reached the sandbox:\n%s", step.Output)
	}
	if e.Steps[1].Outcome != "ok" || e.Steps[1].Output != "403" {
		t.Errorf("other host: step = %+v, want a 403 from egress", e.Steps[1])
	}
	if !strings.Contains(s.logs.String(), "egress connect") {
		t.Errorf("egress did not log the CONNECT:\n%s", s.logs.String())
	}

	// From outside the sandbox, through the same proxy, trusting the CA:
	// the placeholder is dead after the call, and no placeholder at all is
	// a challenge.
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(s.app.ca.PEM())
	proxy, _ := url.Parse(s.egress.URL)
	trusting := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy), TLSClientConfig: &tls.Config{RootCAs: roots}}}
	req, _ := http.NewRequest("GET", "https://"+host+"/demo/echo", nil)
	req.Header.Set("Authorization", "token "+ph)
	res, err := trusting.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("retired placeholder through the proxy: %d, want 401", res.StatusCode)
	}
	res, err = trusting.Get("https://" + host + "/demo/echo")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized || res.Header.Get("WWW-Authenticate") == "" {
		t.Errorf("no placeholder: %d %v, want 401 with a challenge", res.StatusCode, res.Header)
	}
	// The certificate is the platform's, not the host's: a client that does
	// not trust the CA refuses the tunnel before sending anything.
	wary := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}
	if _, err := wary.Get("https://" + host + "/demo/echo"); err == nil {
		t.Errorf("a client that does not trust the platform CA accepted the tunnel")
	}
	assertNoLeak(t, s)
	if strings.Contains(s.logs.String(), ph) {
		t.Errorf("a log line names the placeholder:\n%s", s.logs.String())
	}
}
