import { inputHelp, type AgentOption } from './types'

export interface RunFormProps {
  readonly agents: readonly AgentOption[]
  readonly agentId: string
  readonly input: string
  readonly description: string
  readonly running: boolean
  readonly onAgentChange: (id: string) => void
  readonly onInputChange: (input: string) => void
  readonly onDescriptionChange: (description: string) => void
  readonly onRun: () => void
}

// RunForm picks an agent and the scripted input, one `tool {json}` per line,
// with an optional note on what the run is for.
export function RunForm({ agents, agentId, input, description, running, onAgentChange, onInputChange, onDescriptionChange, onRun }: RunFormProps) {
  return (
    <form
      className="card stack"
      onSubmit={(e) => {
        e.preventDefault()
        onRun()
      }}
    >
      <h2>Run an agent</h2>
      {agents.length === 0 && <p className="muted">No agents yet. Create one on the Agents tab.</p>}
      <label>
        Agent <span className="muted" style={{ fontWeight: 400 }}>runs on its latest version</span>
        <select value={agentId} onChange={(e) => onAgentChange(e.target.value)} required>
          <option value="">Pick an agent</option>
          {agents.map((a) => (
            <option key={a.id} value={a.id}>
              {a.name} (v{a.version})
            </option>
          ))}
        </select>
      </label>
      <label>
        Input <span className="muted" style={{ fontWeight: 400 }}>{inputHelp}</span>
        <textarea className="mono" rows={4} value={input} onChange={(e) => onInputChange(e.target.value)} />
      </label>
      <label>
        Description <span className="muted" style={{ fontWeight: 400 }}>optional: what this run is for and what to expect; shown with it, never sent to the model</span>
        <textarea rows={2} value={description} onChange={(e) => onDescriptionChange(e.target.value)} />
      </label>
      <div className="row">
        <button className="btn btn-primary" type="submit" disabled={running || !agentId}>{running ? 'Running…' : 'Run'}</button>
      </div>
    </form>
  )
}
