import { createContext, useContext, type ReactNode } from 'react'
import type { Tool } from './types'

// ToolsDeps is everything the tools domain needs from outside.
export interface ToolsDeps {
  readonly list: () => Promise<readonly Tool[]>
  readonly put: (t: Tool) => Promise<Tool>
  // credentialRefs names the stored credentials a tool may use.
  readonly credentialRefs: () => Promise<readonly string[]>
}

const Ctx = createContext<ToolsDeps | null>(null)

export function ToolsProvider({ deps, children }: { readonly deps: ToolsDeps; readonly children: ReactNode }) {
  return <Ctx.Provider value={deps}>{children}</Ctx.Provider>
}

export function useToolsDeps(): ToolsDeps {
  const d = useContext(Ctx)
  if (!d) throw new Error('ToolsProvider is missing')
  return d
}
