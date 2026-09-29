export type Kind = 'http' | 'exec'

// Credential names a stored credential and where the tool receives it: an
// HTTP tool gets the real value in a header; an exec tool gets a
// placeholder in an environment variable, exchanged for the real value
// only on a request to one of its hosts.
export interface Credential {
  readonly ref: string
  readonly header?: string
  readonly prefix?: string
  readonly env?: string
  readonly hosts?: readonly string[]
}

export interface Tool {
  readonly name: string
  readonly description: string
  readonly params?: unknown
  readonly kind: Kind
  readonly http?: { readonly method: string; readonly url: string }
  readonly exec?: { readonly argv: readonly string[]; readonly timeoutSeconds?: number }
  readonly credential?: Credential
}

export const emptyHTTPTool: Tool = { name: '', description: '', kind: 'http', http: { method: 'GET', url: 'https://' } }
export const emptyExecTool: Tool = { name: '', description: '', kind: 'exec', exec: { argv: [] } }

export const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'] as const

export function describe(t: Tool): string {
  return t.kind === 'exec' ? (t.exec?.argv ?? []).join(' ') : `${t.http?.url ?? ''}`
}
