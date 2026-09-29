package egress_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/egress"
	"github.com/gaurasha/agentic_orchestration_framework/src/server/internal/egress/deps/memory"
)

const (
	ph     = "fake_0123456789abcdef0123456789abcdef"
	secret = "real-token-value"
)

type upstream struct{ c *http.Client }

func (u upstream) Do(ctx context.Context, r *http.Request) (*http.Response, error) {
	return u.c.Do(r.WithContext(ctx))
}

// 5.2: a request from the sandbox reaches the external service with the
// real credential in place of the placeholder, for that placeholder's host
// only. The response carries the placeholder back, never the real value.
func TestForwardSwapsAndHides(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		w.Header().Set("X-Echo", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"got":"` + r.Header.Get("Authorization") + `","q":"` + r.URL.RawQuery + `"}`))
	}))
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "http://")
	var logs bytes.Buffer
	api := egress.New(egress.Deps{
		Placeholders: memory.Placeholders{ph: {Real: secret, Hosts: []string{host}}},
		Upstream:     upstream{http.DefaultClient},
		Log:          slog.New(slog.NewTextHandler(&logs, nil)),
	}, egress.Config{AllowHTTP: true})
	req := egress.Request{Method: "GET", Scheme: "http", Host: host, Path: "/user", RawQuery: "x=1",
		Header: http.Header{"Authorization": {"Bearer " + ph}}}

	res, err := api.Forward(context.Background(), req)
	if err != nil || res.Status != 200 {
		t.Fatalf("Forward = %+v, %v", res, err)
	}
	if len(seen) != 1 || seen[0] != "Bearer "+secret {
		t.Errorf("service saw %v", seen)
	}
	if string(res.Body) != `{"got":"Bearer `+ph+`","q":"x=1"}` || res.Header.Get("X-Echo") != "Bearer "+ph {
		t.Errorf("response leaks or lacks the placeholder: %s %v", res.Body, res.Header)
	}
	if strings.Contains(logs.String(), secret) || strings.Contains(logs.String(), ph) {
		t.Errorf("log leaks: %s", logs.String())
	}

	cases := []struct {
		name string
		req  egress.Request
		want error
	}{
		{"other host", egress.Request{Method: "GET", Scheme: "http", Host: "other.example", Path: "/", Header: req.Header}, egress.ErrDenied},
		{"unknown placeholder", egress.Request{Method: "GET", Scheme: "http", Host: host, Path: "/", Header: http.Header{"Authorization": {"Bearer fake_ffffffffffffffffffffffffffffffff"}}}, egress.ErrUnauthenticated},
		{"no placeholder", egress.Request{Method: "GET", Scheme: "http", Host: host, Path: "/"}, egress.ErrUnauthenticated},
		{"plain http refused when not allowed", egress.Request{Method: "GET", Scheme: "ftp", Host: host, Path: "/", Header: req.Header}, egress.ErrDenied},
	}
	for _, c := range cases {
		if _, err := api.Forward(context.Background(), c.req); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
	if len(seen) != 1 {
		t.Errorf("a refused request reached the service: %d requests", len(seen))
	}
}

// 5.2: an escaped path segment reaches the host as sent, and a reply over
// the size limit is an error, not a success with a cut body.
func TestEscapedPathAndOversizedReply(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		if r.URL.Query().Get("big") != "" {
			_, _ = w.Write(bytes.Repeat([]byte("x"), 2<<20))
		}
	}))
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "http://")
	api := egress.New(egress.Deps{
		Placeholders: memory.Placeholders{ph: {Real: secret, Hosts: []string{host}}},
		Upstream:     upstream{http.DefaultClient},
		Log:          slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	}, egress.Config{AllowHTTP: true, MaxBody: 1 << 20})
	hdr := http.Header{"Authorization": {"Bearer " + ph}}
	if _, err := api.Forward(context.Background(), egress.Request{Method: "GET", Scheme: "http", Host: host, Path: "/projects/group/name", RawPath: "/projects/group%2Fname", Header: hdr}); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/projects/group%2Fname" {
		t.Errorf("host saw path %q, want the escaped segment kept", gotPath)
	}
	_, err := api.Forward(context.Background(), egress.Request{Method: "GET", Scheme: "http", Host: host, Path: "/", RawQuery: "big=1", Header: hdr})
	if !errors.Is(err, egress.ErrTooLarge) {
		t.Errorf("oversized reply: err = %v, want ErrTooLarge", err)
	}
}
