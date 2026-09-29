package gateway

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// argsOf reads a call's arguments as name -> value, each value as text.
func argsOf(raw json.RawMessage) (map[string]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]string{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%w: arguments must be a JSON object", ErrDenied)
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		switch x := v.(type) {
		case string:
			out[k] = x
		case nil:
			out[k] = ""
		default:
			b, _ := json.Marshal(x)
			out[k] = string(b)
		}
	}
	return out, nil
}

// checkGrant refuses a tool the version does not grant or an argument
// outside the allowlist, and returns the grant. Every refusal names its
// reason. Approval is the caller's to check, after the journal.
func checkGrant(g Grant, tool string, args map[string]string) (ToolGrant, error) {
	tg, ok := g.Tools[tool]
	if !ok {
		return ToolGrant{}, fmt.Errorf("%w: tool %s is not granted to this agent version", ErrDenied, tool)
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if len(tg.Allowlist) == 0 {
			break
		}
		allowed, listed := tg.Allowlist[k]
		if !listed {
			return ToolGrant{}, fmt.Errorf("%w: argument %s is not allowed", ErrDenied, k)
		}
		if !slices.Contains(allowed, args[k]) {
			return ToolGrant{}, fmt.Errorf("%w: argument %s=%q is not allowed", ErrDenied, k, args[k])
		}
	}
	return tg, nil
}

var placeholder = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)

// buildURL fills the template's placeholders with argument values, escaped
// for where they land: a path segment or a query value. The tool registry
// already refused a placeholder in the host.
func buildURL(template string, args map[string]string) (*url.URL, error) {
	var missing []string
	path, query, hasQuery := strings.Cut(template, "?")
	fill := func(part string, escape func(string) string) string {
		return placeholder.ReplaceAllStringFunc(part, func(m string) string {
			name := m[1 : len(m)-1]
			v, ok := args[name]
			if !ok {
				missing = append(missing, name)
				return m
			}
			return escape(v)
		})
	}
	filled := fill(path, url.PathEscape)
	if hasQuery {
		filled += "?" + fill(query, url.QueryEscape)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: missing argument %s", ErrDenied, strings.Join(missing, ", "))
	}
	u, err := url.Parse(filled)
	if err != nil {
		return nil, fmt.Errorf("%w: url: %v", ErrDenied, err)
	}
	return u, nil
}

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// proxyVars are the names clients read for their HTTP proxy: Go reads the
// upper case, curl the lower; both get the egress URL.
var proxyVars = []string{"HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy"}

// commandEnv is the whole environment a command gets: each argument as
// ARG_<name>, the egress URL as EGRESS and as the HTTP proxy, and the
// placeholder where the credential goes. Nothing else, so nothing else
// can leak in.
func commandEnv(args map[string]string, egress, credEnv, placeholder string) (map[string]string, error) {
	env := make(map[string]string, len(args)+2+len(proxyVars))
	for k, v := range args {
		if !envName.MatchString(k) {
			return nil, fmt.Errorf("%w: argument name %q cannot be an environment variable", ErrDenied, k)
		}
		env["ARG_"+k] = v
	}
	env["EGRESS"] = egress
	for _, k := range proxyVars {
		env[k] = egress
	}
	if credEnv != "" {
		env[credEnv] = placeholder
	}
	return env, nil
}

// commandOutput is what the model sees of a command.
func commandOutput(out Output) string {
	s := out.Stdout
	if out.Stderr != "" {
		s += "\n[stderr]\n" + out.Stderr
	}
	return s
}

// redact hides the credential wherever the output repeats it, plain or in
// the encodings a reply commonly uses (JSON-escaped, URL-escaped, base64),
// then caps the output's size.
func redact(body string, secret string, max int) string {
	if secret != "" {
		body = hideForms(body, secret, "[secret]")
	}
	if len(body) > max {
		body = body[:max] + "…[truncated]"
	}
	return body
}

// base64Run matches anything that could be base64, down to one block.
var base64Run = regexp.MustCompile(`[A-Za-z0-9+/_-]{4,}={0,2}`)

// hideForms replaces secret with mask wherever s carries it: as is, as a
// JSON string escapes it, as a URL escapes it, and inside any base64 run
// that decodes to something containing it (an echoed Basic credential).
func hideForms(s, secret, mask string) string {
	s = strings.ReplaceAll(s, secret, mask)
	if j, err := json.Marshal(secret); err == nil {
		if escaped := string(j[1 : len(j)-1]); escaped != secret {
			s = strings.ReplaceAll(s, escaped, mask)
		}
	}
	for _, escaped := range []string{url.QueryEscape(secret), url.PathEscape(secret)} {
		if escaped != secret {
			s = strings.ReplaceAll(s, escaped, mask)
		}
	}
	return base64Run.ReplaceAllStringFunc(s, func(run string) string {
		for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			raw, err := enc.DecodeString(run)
			if err != nil || !strings.Contains(string(raw), secret) {
				continue
			}
			return enc.EncodeToString([]byte(strings.ReplaceAll(string(raw), secret, mask)))
		}
		return run
	})
}
