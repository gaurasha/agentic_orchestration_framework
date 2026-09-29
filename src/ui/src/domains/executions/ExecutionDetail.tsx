import type { Execution, Outcome, Status, Step } from './types'
import { WaitPanel } from './WaitPanel'

export interface ExecutionDetailProps {
  readonly execution: Execution
  readonly agentName: string
  readonly busy: boolean
  readonly onDecide: (approve: boolean, reason: string) => void
  readonly onAnswer: (text: string) => void
  readonly onCancel: (reason: string) => void
  readonly onRetry: (step: number) => void
}

const outcomeClass: Record<Outcome, string> = { ok: 'notice-ok', error: 'notice-error', refused: 'notice-error', rejected: 'notice-error' }
const statusClass: Record<Status, string> = { running: '', waiting: 'badge-accent', succeeded: 'notice-ok', failed: 'notice-error', cancelled: 'notice-error' }

function StepView({ step, busy, onRetry }: { readonly step: Step; readonly busy: boolean; readonly onRetry: () => void }) {
  const isCall = step.kind !== 'question' && step.outcome !== 'rejected' && step.outcome !== 'refused'
  return (
    <div className="tool-grant stack">
      <div className="row-between">
        <span>
          <span className="muted">{step.index}.</span> <code>{step.tool}</code>{' '}
          {step.kind !== 'question' && <span className="mono muted">{JSON.stringify(step.args)}</span>}
        </span>
        <span className="row">
          {step.kind && <span className="badge">{step.kind}</span>}
          {step.exitCode !== undefined && <span className="badge">exit {step.exitCode}</span>}
          {step.approved && <span className="badge badge-accent">approved</span>}
          {step.replayed && <span className="badge badge-accent">replayed: nothing ran</span>}
          <span className={`badge ${outcomeClass[step.outcome]}`}>{step.outcome}</span>
          {isCall && !step.replayed && (
            <button className="btn btn-small" disabled={busy} onClick={onRetry} title="Repeat this call with the same key: the gateway returns the saved result">
              Retry
            </button>
          )}
        </span>
      </div>
      <pre className="output">{step.output}</pre>
    </div>
  )
}

// ExecutionDetail shows one run: the version it was pinned to, what it is
// waiting for, each tool call as the model saw its result, and the final
// answer.
export function ExecutionDetail({ execution: e, agentName, busy, onDecide, onAnswer, onCancel, onRetry }: ExecutionDetailProps) {
  return (
    <section className="card stack">
      <div className="row">
        <h2>{agentName}</h2>
        <span className="badge badge-accent">ran on v{e.agentVersion}</span>
        <span className={`badge ${statusClass[e.status]}`}>{e.status}</span>
      </div>
      <div className="muted mono">{e.id}</div>
      {e.description && (
        <div className="stack">
          <h3>What to expect</h3>
          <p className="description">{e.description}</p>
        </div>
      )}
      {e.wait && <WaitPanel wait={e.wait} busy={busy} onDecide={onDecide} onAnswer={onAnswer} onCancel={onCancel} />}
      <div className="stack">
        <h3>Input</h3>
        <pre className="output">{e.input || <span className="muted">empty</span>}</pre>
      </div>
      <div className="stack">
        <h3>Steps</h3>
        {e.steps.length === 0 && <span className="muted">No steps yet.</span>}
        {e.steps.map((s) => (
          <StepView key={s.index} step={s} busy={busy} onRetry={() => onRetry(s.index)} />
        ))}
      </div>
      {e.status !== 'waiting' && (
        <div className="stack">
          <h3>Final answer</h3>
          <p style={{ margin: 0 }}>{e.error ? <span className="notice notice-error">{e.error}</span> : e.output}</p>
        </div>
      )}
    </section>
  )
}
