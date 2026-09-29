import type { Agent } from './types'

export interface AgentListProps {
  readonly agents: readonly Agent[]
  readonly selected: string | null
  readonly onSelect: (id: string) => void
  readonly onNew: () => void
}

export function AgentList({ agents, selected, onSelect, onNew }: AgentListProps) {
  return (
    <section className="card stack">
      <div className="row-between">
        <h2>Agents</h2>
        <button className="btn btn-small btn-primary" onClick={onNew}>New agent</button>
      </div>
      {agents.length === 0 ? (
        <p className="empty">No agents yet.</p>
      ) : (
        <ul className="list">
          {agents.map((a) => (
            <li key={a.id}>
              <button aria-current={a.id === selected} onClick={() => onSelect(a.id)}>
                <div className="row-between">
                  <strong>{a.name}</strong>
                  <span className="badge">v{a.version}</span>
                </div>
                <div className="muted mono">{a.id}</div>
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
