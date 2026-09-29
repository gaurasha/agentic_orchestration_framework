package egress

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
)

const ph = "fake_0123456789abcdef0123456789abcdef"

func TestFindAndSwapAndHide(t *testing.T) {
	h := http.Header{"Authorization": {"Bearer " + ph}, "X-Other": {"plain"}, "Connection": {"close"}}
	got := findPlaceholders(h)
	if !slices.Equal(got, []string{ph}) {
		t.Errorf("findPlaceholders = %v", got)
	}
	if findPlaceholders(http.Header{"A": {"fake_short", "nothing"}}) != nil {
		t.Error("matched a value that is not a placeholder")
	}

	real := map[string]string{ph: "real-token"}
	swapped := swap(dropHopByHop(h), real)
	if swapped.Get("Authorization") != "Bearer real-token" || swapped.Get("X-Other") != "plain" || swapped.Get("Connection") != "" {
		t.Errorf("swap = %v", swapped)
	}

	body, hdr := hide([]byte(`{"echo":"real-token"}`), http.Header{"X-Echo": {"real-token"}}, real)
	if string(body) != `{"echo":"`+ph+`"}` || hdr.Get("X-Echo") != ph {
		t.Errorf("hide = %s %v", body, hdr)
	}
}

// git and curl -u send the token inside a Basic credential.
func TestBasicCredential(t *testing.T) {
	basic := "Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:"+ph))
	h := http.Header{"Authorization": {basic}}
	if got := findPlaceholders(h); !slices.Equal(got, []string{ph}) {
		t.Fatalf("findPlaceholders = %v", got)
	}
	swapped := swap(h, map[string]string{ph: "real-token"})
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:real-token"))
	if swapped.Get("Authorization") != want {
		t.Errorf("swap = %q, want %q", swapped.Get("Authorization"), want)
	}
	if dropHopByHop(http.Header{"Accept-Encoding": {"gzip"}, "Accept": {"*/*"}}).Get("Accept-Encoding") != "" {
		t.Error("Accept-Encoding was forwarded")
	}
}

// A reply that echoes the Basic credential carries the real value base64
// encoded; the sandbox must get the placeholder back inside that encoding.
func TestHideInsideBase64(t *testing.T) {
	real := map[string]string{ph: "real-token"}
	echoed := base64.StdEncoding.EncodeToString([]byte("x-access-token:real-token"))
	body, hdr := hide([]byte(`{"auth":"Basic `+echoed+`"}`), http.Header{"X-Echo": {"Basic " + echoed}}, real)
	want := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + ph))
	if string(body) != `{"auth":"Basic `+want+`"}` || hdr.Get("X-Echo") != "Basic "+want {
		t.Errorf("hide = %s %v", body, hdr)
	}
	if got := hideValue("plain text with no secret, and a longish base64 run: QUJDREVGR0hJSktMTU5PUA==", real); !strings.Contains(got, "QUJDREVGR0hJSktMTU5PUA==") {
		t.Errorf("an unrelated run was changed: %s", got)
	}
}

// 5.2: the real value is hidden in the encodings a reply commonly uses,
// and a short Basic credential is found too.
func TestHideEncodedForms(t *testing.T) {
	secret := `p@ss"w\rd&x`
	real := map[string]string{ph: secret}
	short := base64.StdEncoding.EncodeToString([]byte("u:" + secret))
	body, _ := json.Marshal(map[string]string{"json": secret, "url": url.QueryEscape(secret), "basic": "Basic " + short})
	jsonForm, _ := json.Marshal(secret)
	got, _ := hide(body, nil, real)
	for name, form := range map[string]string{"json": string(jsonForm[1 : len(jsonForm)-1]), "url": url.QueryEscape(secret), "short basic": short} {
		if strings.Contains(string(got), form) {
			t.Errorf("%s form of the real value survived: %s", name, got)
		}
	}
	var back map[string]string
	if err := json.Unmarshal(got, &back); err != nil {
		t.Fatalf("hiding broke the JSON: %v in %s", err, got)
	}
	if raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(back["basic"], "Basic ")); string(raw) != "u:"+ph {
		t.Errorf("basic decodes to %q, want u:%s", raw, ph)
	}
	if back["json"] != ph || back["url"] != ph {
		t.Errorf("json/url fields = %q, %q, want the placeholder", back["json"], back["url"])
	}
}
