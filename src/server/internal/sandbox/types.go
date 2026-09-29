package sandbox

import "time"

type Command struct {
	Argv    []string
	Env     map[string]string // the whole environment; never a real credential
	Timeout time.Duration
}

type Output struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// MaxOutput caps each of stdout and stderr.
const MaxOutput = 64 << 10
