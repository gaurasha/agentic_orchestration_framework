import { createContext, useContext, type ReactNode } from 'react'
import type { AgentOption, Execution } from './types'

// ExecutionsDeps is everything the executions domain needs from outside.
export interface ExecutionsDeps {
  readonly list: () => Promise<readonly Execution[]>
  readonly start: (agentId: string, input: string, description: string) => Promise<Execution>
  readonly decide: (id: string, key: string, approve: boolean, reason: string) => Promise<Execution>
  readonly answer: (id: string, key: string, text: string) => Promise<Execution>
  readonly cancel: (id: string, reason: string) => Promise<Execution>
  readonly retry: (id: string, step: number) => Promise<Execution>
  readonly agents: () => Promise<readonly AgentOption[]>
}

const Ctx = createContext<ExecutionsDeps | null>(null)

export function ExecutionsProvider({ deps, children }: { readonly deps: ExecutionsDeps; readonly children: ReactNode }) {
  return <Ctx.Provider value={deps}>{children}</Ctx.Provider>
}

export function useExecutionsDeps(): ExecutionsDeps {
  const d = useContext(Ctx)
  if (!d) throw new Error('ExecutionsProvider is missing')
  return d
}
