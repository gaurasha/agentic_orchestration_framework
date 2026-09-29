import { summarize, type Agent, type ToolOption } from './types'

export interface AgentVersionsProps {
  readonly versions: readonly Agent[]
  readonly tools: readonly ToolOption[] // the registry, to say what each grant means
  readonly shown: number
  readonly onShow: (version: number) => void
  readonly onEdit: () => void
}

// AgentVersions shows one version of an agent, read-only, beside its history.
export function AgentVersions({ versions, tools, shown, onShow, onEdit }: AgentVersionsProps) {
  const latest = versions[versions.length - 1]
  const v = versions.find((x) => x.version === shown) ?? latest
  if (!v || !latest) return null
  return (
    <section className="card stack">
      <div className="row-between">
        <div className="row">
          <h2>{v.name}</h2>
          <span className="badge badge-accent">v{v.version}</span>
          {v.version !== latest.version && <span className="badge">not the latest</span>}
        </div>
        <button className="btn btn-primary" onClick={onEdit}>Edit latest</button>
      </div>
      <div className="row muted" style={{ flexWrap: 'wrap' }}>
        <span className="mono">{v.id}</span>·<span>model {v.model}</span>·
        <span>{v.budget.maxCalls ? `${v.budget.maxCalls} tool calls` : 'no call cap'}</span>·
        <span title={v.hash} className="mono">#{v.hash.slice(0, 8)}</span>
      </div>

      <div className="stack">
        <h3>Instructions</h3>
        <p style={{ whiteSpace: 'pre-wrap', margin: 0 }}>{v.instructions || <span className="muted">None</span>}</p>
      </div>

      <div className="stack">
        <h3>Tools</h3>
        {(v.tools ?? []).length === 0 && <span className="muted">None</span>}
        {(v.tools ?? []).map((t) => {
          const tool = tools.find((x) => x.name === t.name)
          const info = tool ? summarize(tool) : null
          const allowed = t.allowlist
            ? Object.entries(t.allowlist)
                .map(([k, vals]) => `${k}: ${vals.join(' or ')}`)
                .join('; ') + '. Any other value is refused before the call.'
            : 'any values'
          return (
            <div key={t.name} className="tool-grant stack">
              <div className="row">
                <code>{t.name}</code>
                {info ? (
                  <span className="badge">{info.kind}</span>
                ) : (
                  <span className="badge notice-error">not in the registry: calls are refused</span>
                )}
                {t.needsApproval && <span className="badge badge-accent">a human approves each call</span>}
              </div>
              {tool?.description && <div>{tool.description}</div>}
              <dl className="facts">
                {info && (
                  <>
                    <dt>{info.doesLabel}</dt>
                    <dd className="mono">{info.does}</dd>
                    <dt>Credential</dt>
                    <dd>{info.credential}</dd>
                    <dt>Arguments</dt>
                    <dd>{info.args}</dd>
                  </>
                )}
                <dt>Allowed values</dt>
                <dd>{allowed}</dd>
                <dt>Approval</dt>
                <dd>{t.needsApproval ? 'a human approves each call; the run waits until then' : 'not needed'}</dd>
              </dl>
            </div>
          )
        })}
      </div>

      <div className="stack">
        <h3>History</h3>
        <ul className="list">
          {[...versions].reverse().map((x) => (
            <li key={x.version}>
              <button aria-current={x.version === v.version} onClick={() => onShow(x.version)}>
                <div className="row-between">
                  <span>
                    <strong>v{x.version}</strong> <span className="muted">{x.name}</span>
                  </span>
                  <span className="muted">{new Date(x.createdAt).toLocaleString()}</span>
                </div>
              </button>
            </li>
          ))}
        </ul>
      </div>
    </section>
  )
}
