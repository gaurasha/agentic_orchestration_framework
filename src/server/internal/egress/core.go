package egress

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// placeholderPattern matches the values the credentials domain issues.
var placeholderPattern = regexp.MustCompile(`fake_[0-9a-f]{32}`)

// findPlaceholders lists each distinct placeholder in the request's
// headers. A Basic credential is decoded first, since git and curl -u put
// the token there.
func findPlaceholders(h http.Header) []string {
	seen := map[string]bool{}
	var out []string
	for _, vs := range h {
		for _, v := range vs {
			for _, m := range placeholderPattern.FindAllString(decodeBasic(v), -1) {
				if !seen[m] {
					seen[m] = true
					out = append(out, m)
				}
			}
		}
	}
	return out
}

// swap replaces each placeholder in the headers with its real value, and
// returns the result as a new header set. A Basic credential is swapped
// inside its encoding.
func swap(h http.Header, real map[string]string) http.Header {
	out := make(http.Header, len(h))
	for k, vs := range h {
		for _, v := range vs {
			out[k] = append(out[k], swapValue(v, real))
		}
	}
	return out
}

func swapValue(v string, real map[string]string) string {
	if user, ok := strings.CutPrefix(v, "Basic "); ok {
		if raw, err := base64.StdEncoding.DecodeString(user); err == nil {
			return "Basic " + base64.StdEncoding.EncodeToString([]byte(swapValue(string(raw), real)))
		}
	}
	for ph, r := range real {
		v = strings.ReplaceAll(v, ph, r)
	}
	return v
}

// decodeBasic returns a Basic credential's decoded form, or the value as is.
func decodeBasic(v string) string {
	if user, ok := strings.CutPrefix(v, "Basic "); ok {
		if raw, err := base64.StdEncoding.DecodeString(user); err == nil {
			return string(raw)
		}
	}
	return v
}

// hide puts each placeholder back wherever the response carries the real
// value, so the sandbox never sees it even if the service echoes it.
func hide(body []byte, h http.Header, real map[string]string) ([]byte, http.Header) {
	out := make(http.Header, len(h))
	for k, vs := range h {
		for _, v := range vs {
			out[k] = append(out[k], hideValue(v, real))
		}
	}
	return []byte(hideValue(string(body), real)), out
}

// base64Run matches anything that could be base64, down to one block, so a
// short Basic credential is scanned too.
var base64Run = regexp.MustCompile(`[A-Za-z0-9+/_-]{4,}={0,2}`)

// hideValue puts the placeholder back wherever v carries the real value:
// as is, as a JSON string escapes it, as a URL escapes it, and inside any
// base64 run that decodes to something containing it (an echoed Basic
// credential). The placeholder has no special characters, so it reads the
// same in every form.
func hideValue(v string, real map[string]string) string {
	for ph, r := range real {
		v = strings.ReplaceAll(v, r, ph)
		if j, err := json.Marshal(r); err == nil {
			if escaped := string(j[1 : len(j)-1]); escaped != r {
				v = strings.ReplaceAll(v, escaped, ph)
			}
		}
		for _, escaped := range []string{url.QueryEscape(r), url.PathEscape(r)} {
			if escaped != r {
				v = strings.ReplaceAll(v, escaped, ph)
			}
		}
	}
	return base64Run.ReplaceAllStringFunc(v, func(run string) string {
		for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			raw, err := enc.DecodeString(run)
			if err != nil {
				continue
			}
			hidden := string(raw)
			for ph, r := range real {
				hidden = strings.ReplaceAll(hidden, r, ph)
			}
			if hidden == string(raw) {
				return run
			}
			return enc.EncodeToString([]byte(hidden))
		}
		return run
	})
}

// hopByHop are connection headers a forwarder must drop. Accept-Encoding
// goes too, so the reply arrives readable and the real value can be hidden
// in it; Go's client asks for gzip on its own and unpacks it.
var hopByHop = map[string]bool{
	"Connection": true, "Keep-Alive": true, "Proxy-Authenticate": true, "Proxy-Authorization": true,
	"Te": true, "Trailer": true, "Transfer-Encoding": true, "Upgrade": true, "Host": true, "Content-Length": true,
	"Accept-Encoding": true,
}

func dropHopByHop(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, vs := range h {
		if !hopByHop[http.CanonicalHeaderKey(k)] {
			out[k] = vs
		}
	}
	return out
}
