package main

// The /v1 API the UI calls, and /demo/echo, the fake external API the demo
// tool calls. Errors go back as {"error": "..."}.

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/agents"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/credentials"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/executions"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/tools"
)

const tokenTTL = 12 * time.Hour

func (a *app) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	mux.HandleFunc("GET /demo/echo", a.demoEcho)
	mux.HandleFunc("GET /demo/git/{tenant}/{ref}/{rest...}", a.demoGit)
	mux.HandleFunc("POST /v1/token", a.mintToken)

	api := http.NewServeMux()
	api.HandleFunc("GET /v1/agents", a.listAgents)
	api.HandleFunc("POST /v1/agents", a.createAgent)
	api.HandleFunc("PUT /v1/agents/{id}", a.updateAgent)
	api.HandleFunc("GET /v1/agents/{id}/versions", a.agentVersions)
	api.HandleFunc("GET /v1/tools", a.listTools)
	api.HandleFunc("PUT /v1/tools/{name}", a.putTool)
	api.HandleFunc("GET /v1/credentials", a.listCredentials)
	api.HandleFunc("PUT /v1/credentials/{ref}", a.storeCredential)
	api.HandleFunc("GET /v1/executions", a.listExecutions)
	api.HandleFunc("POST /v1/executions", a.startExecution)
	api.HandleFunc("GET /v1/executions/{id}", a.getExecution)
	api.HandleFunc("POST /v1/executions/{id}/decide", a.decide)
	api.HandleFunc("POST /v1/executions/{id}/answer", a.answer)
	api.HandleFunc("POST /v1/executions/{id}/cancel", a.cancel)
	api.HandleFunc("POST /v1/executions/{id}/steps/{step}/retry", a.retry)
	mux.Handle("/v1/", requireTenant(tenantTokensOf{a.credentials}, api))
	return mux
}

// demoEcho is the fake external service: it reports the credential header
// and the query it received, and numbers each request, so the demo shows
// what the gateway sent and that a replayed call sent nothing.
func (a *app) demoEcho(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": "demo echo",
		"request": a.echoCount.Add(1),
		"query":   r.URL.Query(),
		"headers": map[string]string{"Authorization": r.Header.Get("Authorization")},
	})
}

// demoGit is a fake git server, enough for `git ls-remote`. It advertises
// its branches only if the Basic credential it receives is the real value
// stored under {tenant}/{ref}; a placeholder, or nothing, gets 401 as
// GitHub would give it. So a real git in the sandbox, holding only the
// placeholder, listing the branch proves egress swapped in the real value.
func (a *app) demoGit(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.PathValue("rest"), "/info/refs") {
		writeError(w, http.StatusNotFound, "only info/refs is served")
		return
	}
	_, pass, ok := r.BasicAuth()
	sec, err := a.credentials.Resolve(r.Context(), r.PathValue("tenant"), r.PathValue("ref"))
	if !ok || err != nil || subtle.ConstantTimeCompare([]byte(pass), []byte(sec.Reveal())) != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="demo git"`)
		writeError(w, http.StatusUnauthorized, "the credential received is not the stored value")
		return
	}
	a.gitCount.Add(1)
	sha := strings.Repeat("a", 40)
	var b bytes.Buffer
	pkt := func(line string) { fmt.Fprintf(&b, "%04x%s", len(line)+4, line) }
	pkt("# service=git-upload-pack\n")
	b.WriteString("0000")
	pkt(sha + " HEAD\x00symref=HEAD:refs/heads/main\n")
	pkt(sha + " refs/heads/main\n")
	pkt(sha + " refs/heads/real-credential-received\n")
	b.WriteString("0000")
	w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
	_, _ = w.Write(b.Bytes())
}

func (a *app) decide(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	var d executions.Decision
	if !readJSON(w, r, &d) {
		return
	}
	out, err := a.executions.Decide(r.Context(), tenant, r.PathValue("id"), d)
	a.reply(w, http.StatusOK, out, err)
}

func (a *app) answer(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	var in executions.Reply
	if !readJSON(w, r, &in) {
		return
	}
	out, err := a.executions.Answer(r.Context(), tenant, r.PathValue("id"), in)
	a.reply(w, http.StatusOK, out, err)
}

func (a *app) cancel(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	out, err := a.executions.Cancel(r.Context(), tenant, r.PathValue("id"), in.Reason)
	a.reply(w, http.StatusOK, out, err)
}

func (a *app) retry(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	step, err := strconv.Atoi(r.PathValue("step"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "step must be a number")
		return
	}
	out, err := a.executions.Retry(r.Context(), tenant, r.PathValue("id"), step)
	a.reply(w, http.StatusOK, out, err)
}

func (a *app) mintToken(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Tenant string `json:"tenant"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	tok, err := a.credentials.Issue(r.Context(), in.Tenant, tokenTTL)
	if err != nil {
		a.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": tok, "tenant": in.Tenant})
}

func (a *app) listAgents(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	out, err := a.agents.List(r.Context(), tenant)
	a.reply(w, http.StatusOK, nonNil(out), err)
}

func (a *app) createAgent(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	var def agents.Definition
	if !readJSON(w, r, &def) {
		return
	}
	out, err := a.agents.Create(r.Context(), tenant, def)
	a.reply(w, http.StatusCreated, out, err)
}

func (a *app) updateAgent(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	var def agents.Definition
	if !readJSON(w, r, &def) {
		return
	}
	out, err := a.agents.Update(r.Context(), tenant, r.PathValue("id"), def)
	a.reply(w, http.StatusOK, out, err)
}

func (a *app) agentVersions(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	out, err := a.agents.Versions(r.Context(), tenant, r.PathValue("id"))
	a.reply(w, http.StatusOK, out, err)
}

func (a *app) listTools(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	out, err := a.tools.List(r.Context(), tenant)
	a.reply(w, http.StatusOK, nonNil(out), err)
}

func (a *app) putTool(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	var t tools.Tool
	if !readJSON(w, r, &t) {
		return
	}
	t.Tenant, t.Name = tenant, r.PathValue("name")
	out, err := a.tools.Put(r.Context(), t)
	a.reply(w, http.StatusOK, out, err)
}

func (a *app) listCredentials(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	out, err := a.credentials.List(r.Context(), tenant)
	a.reply(w, http.StatusOK, nonNil(out), err)
}

func (a *app) storeCredential(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	var in struct {
		Value string `json:"value"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	err := a.credentials.Store(r.Context(), tenant, r.PathValue("ref"), credentials.NewSecret(in.Value))
	a.reply(w, http.StatusNoContent, nil, err)
}

func (a *app) listExecutions(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	out, err := a.executions.List(r.Context(), tenant)
	a.reply(w, http.StatusOK, nonNil(out), err)
}

func (a *app) startExecution(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	var in struct {
		AgentID     string `json:"agentId"`
		Input       string `json:"input"`
		Description string `json:"description"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	out, err := a.executions.Start(r.Context(), tenant, executions.Run{AgentID: in.AgentID, Input: in.Input, Description: in.Description})
	a.reply(w, http.StatusCreated, out, err)
}

func (a *app) getExecution(w http.ResponseWriter, r *http.Request) {
	tenant, ok := tenant(w, r)
	if !ok {
		return
	}
	out, err := a.executions.Get(r.Context(), tenant, r.PathValue("id"))
	a.reply(w, http.StatusOK, out, err)
}

// tenant reads the request's tenant; a missing one is a wiring bug, refused.
func tenant(w http.ResponseWriter, r *http.Request) (string, bool) {
	t, ok := tenantOf(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no tenant")
	}
	return t, ok
}

func (a *app) reply(w http.ResponseWriter, status int, body any, err error) {
	if err != nil {
		a.fail(w, err)
		return
	}
	if body == nil {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, body)
}

// fail maps a domain error to a status. The message is the error's own,
// which names its reason and never a credential.
func (a *app) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agents.ErrNotFound), errors.Is(err, tools.ErrNotFound),
		errors.Is(err, credentials.ErrNotFound), errors.Is(err, executions.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, agents.ErrInvalid), errors.Is(err, tools.ErrInvalid),
		errors.Is(err, credentials.ErrInvalid), errors.Is(err, executions.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, agents.ErrConflict), errors.Is(err, executions.ErrState):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, credentials.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, err.Error())
	default:
		a.log.Error("request failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad JSON: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// nonNil turns a nil slice into [] on the wire.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
