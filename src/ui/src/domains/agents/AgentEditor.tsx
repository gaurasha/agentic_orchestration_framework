import type { Definition, ToolGrant, ToolOption } from './types'

export interface AgentEditorProps {
  readonly title: string
  readonly def: Definition
  readonly tools: readonly ToolOption[]
  readonly saving: boolean
  readonly onChange: (def: Definition) => void
  readonly onSave: () => void
  readonly onCancel: () => void
}

// Allowlists are edited as one line per argument: "arg: value1, value2".
export function allowlistToText(a: ToolGrant['allowlist']): string {
  return Object.entries(a ?? {})
    .map(([arg, vals]) => `${arg}: ${vals.join(', ')}`)
    .join('\n')
}

export function textToAllowlist(text: string): Record<string, string[]> | undefined {
  const out: Record<string, string[]> = {}
  for (const line of text.split('\n')) {
    const i = line.indexOf(':')
    if (i < 0) continue
    const arg = line.slice(0, i).trim()
    out[arg] = line
      .slice(i + 1)
      .split(',')
      .map((v) => v.trim())
      .filter(Boolean)
  }
  return Object.keys(out).length ? out : undefined
}

export function AgentEditor({ title, def, tools, saving, onChange, onSave, onCancel }: AgentEditorProps) {
  const set = (patch: Partial<Definition>) => onChange({ ...def, ...patch })
  const grant = (name: string) => def.tools.find((t) => t.name === name)
  const setGrant = (name: string, g: ToolGrant | null) =>
    set({ tools: g ? [...def.tools.filter((t) => t.name !== name), g] : def.tools.filter((t) => t.name !== name) })

  return (
    <form
      className="card stack"
      onSubmit={(e) => {
        e.preventDefault()
        onSave()
      }}
    >
      <h2>{title}</h2>
      <label>
        Name
        <input value={def.name} onChange={(e) => set({ name: e.target.value })} required />
      </label>
      <label>
        Model
        <input value={def.model} onChange={(e) => set({ model: e.target.value })} required />
      </label>
      <label>
        Instructions
        <textarea value={def.instructions} onChange={(e) => set({ instructions: e.target.value })} rows={5} />
      </label>
      <label style={{ maxWidth: 200 }}>
        Tool calls per execution
        <input
          type="number"
          min={0}
          value={def.budget.maxCalls}
          onChange={(e) => set({ budget: { maxCalls: Number(e.target.value) } })}
        />
        <span className="muted" style={{ fontWeight: 400 }}>0 means no cap</span>
      </label>

      <div className="stack">
        <h3>Tools</h3>
        {tools.length === 0 && <p className="muted">The registry has no tools yet. Add one on the Tools tab.</p>}
        {tools.map((t) => {
          const g = grant(t.name)
          return (
            <div key={t.name} className="tool-grant stack">
              <label className="inline">
                <input
                  type="checkbox"
                  checked={!!g}
                  onChange={(e) => setGrant(t.name, e.target.checked ? { name: t.name, needsApproval: false } : null)}
                />
                <span>
                  <code>{t.name}</code> <span className="muted">{t.description}</span>
                </span>
              </label>
              {g && (
                <>
                  <label className="inline">
                    <input
                      type="checkbox"
                      checked={g.needsApproval}
                      onChange={(e) => setGrant(t.name, { ...g, needsApproval: e.target.checked })}
                    />
                    A human approves each call
                  </label>
                  <label>
                    Allowed arguments
                    <textarea
                      className="mono"
                      rows={2}
                      placeholder="city: London, Paris   (empty allows any arguments)"
                      defaultValue={allowlistToText(g.allowlist)}
                      onBlur={(e) => setGrant(t.name, { ...g, allowlist: textToAllowlist(e.target.value) })}
                    />
                  </label>
                </>
              )}
            </div>
          )
        })}
        {def.tools
          .filter((g) => !tools.some((t) => t.name === g.name))
          .map((g) => (
            <div key={g.name} className="notice notice-error row-between">
              <span>
                <code>{g.name}</code> is no longer in the registry; saving will fail until it is removed.
              </span>
              <button type="button" className="btn btn-small" onClick={() => setGrant(g.name, null)}>Remove</button>
            </div>
          ))}
      </div>

      <div className="row">
        <button className="btn btn-primary" type="submit" disabled={saving}>{saving ? 'Saving…' : 'Save'}</button>
        <button className="btn" type="button" onClick={onCancel}>Cancel</button>
      </div>
    </form>
  )
}
