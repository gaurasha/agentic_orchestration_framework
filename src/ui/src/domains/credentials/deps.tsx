import { createContext, useContext, type ReactNode } from 'react'

// CredentialsDeps is everything the credentials domain needs. Values are
// write-only: nothing here can read one back.
export interface CredentialsDeps {
  readonly refs: () => Promise<readonly string[]>
  readonly store: (ref: string, value: string) => Promise<void>
}

const Ctx = createContext<CredentialsDeps | null>(null)

export function CredentialsProvider({ deps, children }: { readonly deps: CredentialsDeps; readonly children: ReactNode }) {
  return <Ctx.Provider value={deps}>{children}</Ctx.Provider>
}

export function useCredentialsDeps(): CredentialsDeps {
  const d = useContext(Ctx)
  if (!d) throw new Error('CredentialsProvider is missing')
  return d
}
