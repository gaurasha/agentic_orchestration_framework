import { emptyExecTool, emptyHTTPTool, methods, type Credential, type Kind, type Tool } from './types'

export interface ToolEditorProps {
  readonly tool: Tool
  readonly isNew: boolean
  readonly params: string
  readonly credentialRefs: readonly string[]
  readonly saving: boolean
  readonly onChange: (t: Tool) => void
  readonly onParamsChange: (text: string) => void
  readonly onSave: () => void
  readonly onCancel: () => void
}

const hint = { fontWeight: 400 } as const

export function ToolEditor(p: ToolEditorProps) {
  const t = p.tool
  const set = (patch: Partial<Tool>) => p.onChange({ ...t, ...patch })
  const setKind = (kind: Kind) => {
    const base = kind === 'exec' ? emptyExecTool : emptyHTTPTool
    p.onChange({ ...base, name: t.name, description: t.description, params: t.params })
  }
  const setCredential = (patch: Partial<Credential>) => {
    if (t.credential) set({ credential: { ...t.credential, ...patch } })
  }
  const pickCredential = (ref: string) => {
    if (!ref) return set({ credential: undefined })
    const defaults: Credential =
      t.kind === 'exec' ? { ref, env: 'API_TOKEN', hosts: [] } : { ref, header: 'Authorization', prefix: 'Bearer ' }
    set({ credential: { ...defaults, ...t.credential, ref } })
  }

  return (
    <form
      className="card stack"
      onSubmit={(e) => {
        e.preventDefault()
        p.onSave()
      }}
    >
      <h2>{p.isNew ? 'New tool' : `Edit ${t.name}`}</h2>
      <div className="row" style={{ alignItems: 'end' }}>
        <label>
          Kind
          <select value={t.kind} disabled={!p.isNew} onChange={(e) => setKind(e.target.value as Kind)}>
            <option value="http">HTTP request</option>
            <option value="exec">Command in a sandbox</option>
          </select>
        </label>
        <label style={{ flex: 1 }}>
          Name
          <input value={t.name} disabled={!p.isNew} onChange={(e) => set({ name: e.target.value })} placeholder="get_weather" required />
        </label>
      </div>
      <label>
        Description <span className="muted" style={hint}>shown to the model</span>
        <input value={t.description} onChange={(e) => set({ description: e.target.value })} />
      </label>

      {t.kind === 'http' && t.http && (
        <div className="row" style={{ alignItems: 'end' }}>
          <label>
            Method
            <select value={t.http.method} onChange={(e) => set({ http: { ...t.http!, method: e.target.value } })}>
              {methods.map((m) => (
                <option key={m}>{m}</option>
              ))}
            </select>
          </label>
          <label style={{ flex: 1 }}>
            URL <span className="muted" style={hint}>{'{arg} placeholders in the path or query only'}</span>
            <input className="mono" value={t.http.url} onChange={(e) => set({ http: { ...t.http!, url: e.target.value } })} required />
          </label>
        </div>
      )}
      {t.kind === 'exec' && t.exec && (
        <label>
          Command <span className="muted" style={hint}>one argument per line; arguments arrive as $ARG_name, egress as $EGRESS</span>
          <textarea
            className="mono"
            rows={4}
            defaultValue={t.exec.argv.join('\n')}
            placeholder={'sh\n-c\ncurl -sS -H "Authorization: Bearer $GH_TOKEN" "$EGRESS/https/api.github.com/user"'}
            onBlur={(e) => set({ exec: { ...t.exec!, argv: e.target.value.split('\n').filter((l) => l.trim() !== '') } })}
            required
          />
        </label>
      )}
      <label>
        Arguments (JSON Schema)
        <textarea
          className="mono"
          rows={4}
          value={p.params}
          onChange={(e) => p.onParamsChange(e.target.value)}
          placeholder='{"type":"object","properties":{"city":{"type":"string"}}}'
        />
      </label>

      <div className="stack">
        <h3>Credential</h3>
        <label>
          Stored credential
          <select value={t.credential?.ref ?? ''} onChange={(e) => pickCredential(e.target.value)}>
            <option value="">None: the tool needs no credential</option>
            {p.credentialRefs.map((r) => (
              <option key={r}>{r}</option>
            ))}
          </select>
        </label>
        {t.credential && t.kind === 'http' && (
          <div className="row">
            <label style={{ flex: 1 }}>
              Header
              <input value={t.credential.header ?? ''} onChange={(e) => setCredential({ header: e.target.value })} required />
            </label>
            <label style={{ flex: 1 }}>
              Prefix
              <input value={t.credential.prefix ?? ''} onChange={(e) => setCredential({ prefix: e.target.value })} />
            </label>
          </div>
        )}
        {t.credential && t.kind === 'exec' && (
          <div className="row">
            <label style={{ flex: 1 }}>
              Environment variable <span className="muted" style={hint}>holds a placeholder</span>
              <input className="mono" value={t.credential.env ?? ''} onChange={(e) => setCredential({ env: e.target.value })} required />
            </label>
            <label style={{ flex: 2 }}>
              Hosts <span className="muted" style={hint}>comma-separated; the placeholder is exchanged for these only</span>
              <input
                className="mono"
                defaultValue={(t.credential.hosts ?? []).join(', ')}
                onBlur={(e) => setCredential({ hosts: e.target.value.split(',').map((h) => h.trim()).filter(Boolean) })}
                required
              />
            </label>
          </div>
        )}
        <p className="muted" style={{ margin: 0 }}>
          {t.kind === 'exec'
            ? 'The command sees only a placeholder. Egress exchanges it for the real value on requests to the listed hosts.'
            : 'The gateway adds the credential to the request; the agent never sees it.'}
        </p>
      </div>

      <div className="row">
        <button className="btn btn-primary" type="submit" disabled={p.saving}>{p.saving ? 'Saving…' : 'Save tool'}</button>
        <button className="btn" type="button" onClick={p.onCancel}>Cancel</button>
      </div>
    </form>
  )
}
