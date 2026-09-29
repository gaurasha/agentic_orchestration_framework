import { createContext, useContext, type ReactNode } from 'react'
import type { Agent, Definition, ToolOption } from './types'

// AgentsDeps is everything the agents domain needs from outside.
export interface AgentsDeps {
  readonly list: () => Promise<readonly Agent[]>
  readonly create: (def: Definition) => Promise<Agent>
  readonly update: (id: string, def: Definition) => Promise<Agent>
  readonly versions: (id: string) => Promise<readonly Agent[]>
  readonly tools: () => Promise<readonly ToolOption[]>
}

const Ctx = createContext<AgentsDeps | null>(null)

export function AgentsProvider({ deps, children }: { readonly deps: AgentsDeps; readonly children: ReactNode }) {
  return <Ctx.Provider value={deps}>{children}</Ctx.Provider>
}

export function useAgentsDeps(): AgentsDeps {
  const d = useContext(Ctx)
  if (!d) throw new Error('AgentsProvider is missing')
  return d
}
