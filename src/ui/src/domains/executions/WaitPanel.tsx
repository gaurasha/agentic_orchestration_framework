import { useState } from 'react'
import type { Wait } from './types'

export interface WaitPanelProps {
  readonly wait: Wait
  readonly busy: boolean
  readonly onDecide: (approve: boolean, reason: string) => void
  readonly onAnswer: (text: string) => void
  readonly onCancel: (reason: string) => void
}

// WaitPanel asks the human what the execution is waiting for: a decision
// on a tool call, or an answer to the model's question. Either can be
// cancelled instead.
export function WaitPanel({ wait, busy, onDecide, onAnswer, onCancel }: WaitPanelProps) {
  const [text, setText] = useState('')
  return (
    <div className="card stack wait">
      {wait.kind === 'approval' ? (
        <>
          <h3>Waiting for your approval</h3>
          <p style={{ margin: 0 }}>
            The agent wants to call <code>{wait.tool}</code> <span className="mono muted">{JSON.stringify(wait.args)}</span>. Nothing runs until you decide.
          </p>
          <label>
            Reason <span className="muted" style={{ fontWeight: 400 }}>shown to the model if you reject</span>
            <input value={text} onChange={(e) => setText(e.target.value)} placeholder="not on a Friday" />
          </label>
          <div className="row">
            <button className="btn btn-primary" disabled={busy} onClick={() => onDecide(true, text)}>Approve</button>
            <button className="btn" disabled={busy} onClick={() => onDecide(false, text)}>Reject</button>
            <button className="btn" disabled={busy} onClick={() => onCancel(text || 'cancelled from the UI')}>Cancel the run</button>
          </div>
        </>
      ) : (
        <>
          <h3>The agent asks</h3>
          <p style={{ margin: 0 }}>{wait.question}</p>
          <form
            className="row"
            onSubmit={(e) => {
              e.preventDefault()
              onAnswer(text)
            }}
          >
            <input style={{ flex: 1 }} value={text} onChange={(e) => setText(e.target.value)} placeholder="your answer" autoFocus />
            <button className="btn btn-primary" type="submit" disabled={busy}>Answer</button>
            <button className="btn" type="button" disabled={busy} onClick={() => onCancel('cancelled from the UI')}>Cancel the run</button>
          </form>
        </>
      )}
    </div>
  )
}
