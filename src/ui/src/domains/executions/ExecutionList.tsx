import type { AgentOption, Execution } from './types'

export interface ExecutionListProps {
  readonly executions: readonly Execution[]
  readonly agents: readonly AgentOption[]
  readonly selected: string | null
  readonly onSelect: (id: string) => void
}

export function ExecutionList({ executions, agents, selected, onSelect }: ExecutionListProps) {
  const nameOf = (id: string) => agents.find((a) => a.id === id)?.name ?? id
  return (
    <section className="card stack">
      <h2>Runs</h2>
      {executions.length === 0 ? (
        <p className="empty">No runs yet.</p>
      ) : (
        <ul className="list">
          {executions.map((e) => (
            <li key={e.id}>
              <button aria-current={e.id === selected} onClick={() => onSelect(e.id)}>
                <div className="row-between">
                  <strong>{nameOf(e.agentId)}</strong>
                  <span className="badge badge-accent">v{e.agentVersion}</span>
                </div>
                <div className="row-between muted">
                  <span>
                    {e.steps.length} step{e.steps.length === 1 ? '' : 's'} ·{' '}
                    {e.status === 'waiting' ? <strong>waiting for you</strong> : e.status}
                  </span>
                  <span>{new Date(e.startedAt).toLocaleTimeString()}</span>
                </div>
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
