package sandbox

import "time"

type Command struct {
	Argv    []string
	Env     map[string]string
	Timeout time.Duration
}

type Output struct {
	ExitCode int
	Stdout   string
	Stderr   string
}
