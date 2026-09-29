export type Outcome = 'ok' | 'error' | 'refused' | 'rejected'

export type Status = 'running' | 'waiting' | 'succeeded' | 'failed' | 'cancelled'

// Step is one tool call, as the model saw its result, or a question and
// its answer.
export interface Step {
  readonly index: number
  readonly tool: string
  readonly kind?: 'http' | 'exec' | 'question'
  readonly args: unknown
  readonly outcome: Outcome
  readonly exitCode?: number
  readonly output: string
  readonly replayed?: boolean
  readonly approved?: boolean
  readonly at: string
}

// Wait is what a waiting execution needs from a human.
export interface Wait {
  readonly key: string // names this wait; a decision or answer carries it, so a stale one is refused
  readonly kind: 'approval' | 'question'
  readonly tool?: string
  readonly args?: unknown
  readonly question?: string
  readonly askedAt: string
}

export interface Execution {
  readonly id: string
  readonly agentId: string
  readonly agentVersion: number
  readonly input: string
  readonly description?: string // a human's note on the run: what it is for and what to expect
  readonly status: Status
  readonly wait?: Wait
  readonly steps: readonly Step[]
  readonly output: string
  readonly error?: string
  readonly startedAt: string
  readonly endedAt: string
}

// AgentOption is an agent that can be run, at its latest version.
export interface AgentOption {
  readonly id: string
  readonly name: string
  readonly version: number
}

export const exampleInput = ['echo {"city":"London"}', 'echo {"city":"Tokyo"}', 'delete_repo {}'].join('\n')

export const inputHelp = 'one line per step: `tool {json args}` calls a tool, `ask <question>` asks you and waits'
