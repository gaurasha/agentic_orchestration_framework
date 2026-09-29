package egress

import "net/http"

// Request is what the sandbox sent, addressed to an external host.
type Request struct {
	Method   string
	Scheme   string // https, or http where Config allows it
	Host     string
	Path     string
	RawPath  string // the path as sent, escapes kept, when Path alone would lose them (%2F)
	RawQuery string
	Header   http.Header // placeholders in place of credentials
	Body     []byte
}

// Response is the external service's answer, with the real credential
// hidden wherever it appeared.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

type Config struct {
	MaxBody   int  // bytes of a request or response body
	AllowHTTP bool // for local stand-ins of external services; production is https only
}
