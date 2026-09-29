export interface ToolGrant {
  readonly name: string
  readonly allowlist?: Readonly<Record<string, readonly string[]>>
  readonly needsApproval: boolean
}

export interface Definition {
  readonly name: string
  readonly instructions: string
  readonly model: string
  readonly tools: readonly ToolGrant[]
  readonly budget: { readonly maxCalls: number }
}

// Agent is one version of an agent.
export interface Agent extends Definition {
  readonly id: string
  readonly version: number
  readonly hash: string
  readonly createdAt: string
}

// ToolOption is a registry tool an agent may be granted, with enough of
// its definition to show what a grant means without opening the tool.
export interface ToolOption {
  readonly name: string
  readonly description: string
  readonly kind: 'http' | 'exec'
  readonly params?: { readonly properties?: Readonly<Record<string, unknown>>; readonly required?: readonly string[] }
  readonly http?: { readonly method: string; readonly url: string }
  readonly exec?: { readonly argv: readonly string[]; readonly timeoutSeconds?: number }
  readonly credential?: {
    readonly ref: string
    readonly header?: string
    readonly prefix?: string
    readonly env?: string
    readonly hosts?: readonly string[]
  }
}

// summarize describes a tool for a grant card: what it does, how its
// credential travels, and which arguments its schema names.
export function summarize(t: ToolOption): {
  readonly kind: string
  readonly doesLabel: string
  readonly does: string
  readonly credential: string
  readonly args: string
} {
  const isExec = t.kind === 'exec'
  const does = isExec ? (t.exec?.argv ?? []).join(' ') : `${t.http?.method ?? 'GET'} ${t.http?.url ?? ''}`
  const c = t.credential
  const credential = !c
    ? 'none'
    : isExec
      ? `${c.ref}, as a placeholder in $${c.env ?? '?'}; egress exchanges it for the real value on requests to ${(c.hosts ?? []).join(', ') || 'no host'} only`
      : `${c.ref}, added by the gateway as the header ${c.header ?? '?'}: ${c.prefix ?? ''}…; the agent never sees it`
  const props = Object.keys(t.params?.properties ?? {})
  const required = new Set(t.params?.required ?? [])
  const args =
    props.length === 0
      ? 'no schema: any arguments pass'
      : props.map((p) => (required.has(p) ? `${p} (required)` : p)).join(', ') + '; checked against the tool\'s schema before the call'
  return { kind: isExec ? 'command in a sandbox' : 'HTTP request', doesLabel: isExec ? 'Command' : 'Request', does, credential, args }
}

export const emptyDefinition: Definition = { name: '', instructions: '', model: 'mock', tools: [], budget: { maxCalls: 10 } }

export function definitionOf(a: Agent): Definition {
  return { name: a.name, instructions: a.instructions, model: a.model, tools: a.tools ?? [], budget: a.budget }
}
