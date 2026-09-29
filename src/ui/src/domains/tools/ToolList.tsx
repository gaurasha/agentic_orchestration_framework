import { describe, type Tool } from './types'

export interface ToolListProps {
  readonly tools: readonly Tool[]
  readonly selected: string | null
  readonly onSelect: (name: string) => void
  readonly onNew: () => void
}

export function ToolList({ tools, selected, onSelect, onNew }: ToolListProps) {
  return (
    <section className="card stack">
      <div className="row-between">
        <h2>Tool registry</h2>
        <button className="btn btn-small btn-primary" onClick={onNew}>New tool</button>
      </div>
      {tools.length === 0 ? (
        <p className="empty">No tools yet.</p>
      ) : (
        <ul className="list">
          {tools.map((t) => (
            <li key={t.name}>
              <button aria-current={t.name === selected} onClick={() => onSelect(t.name)}>
                <div className="row-between">
                  <code>{t.name}</code>
                  <span className="badge">{t.kind === 'exec' ? 'exec' : t.http?.method}</span>
                </div>
                <div className="muted mono" style={{ overflowWrap: 'anywhere' }}>{describe(t)}</div>
                {t.credential && (
                  <div className="muted">
                    uses {t.credential.ref}
                    {t.kind === 'exec' && ` as $${t.credential.env} for ${(t.credential.hosts ?? []).join(', ')}`}
                  </div>
                )}
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
