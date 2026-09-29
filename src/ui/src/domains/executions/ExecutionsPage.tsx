import { useCallback, useEffect, useState } from 'react'
import { useExecutionsDeps } from './deps'
import { ExecutionDetail } from './ExecutionDetail'
import { ExecutionList } from './ExecutionList'
import { RunForm } from './RunForm'
import { exampleInput, type AgentOption, type Execution } from './types'

// ExecutionsPage holds the page's state; the components it renders are pure.
export function ExecutionsPage() {
  const deps = useExecutionsDeps()
  const [agents, setAgents] = useState<readonly AgentOption[]>([])
  const [executions, setExecutions] = useState<readonly Execution[]>([])
  const [agentId, setAgentId] = useState('')
  const [input, setInput] = useState(exampleInput)
  const [description, setDescription] = useState('')
  const [selected, setSelected] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    const [as, es] = await Promise.all([deps.agents(), deps.list()])
    setAgents(as)
    setExecutions(es)
    setAgentId((id) => id || as[0]?.id || '')
  }, [deps])

  useEffect(() => {
    refresh().catch((e: Error) => setNotice(e.message))
  }, [refresh])

  // A running execution saves each step as it completes; poll to show
  // them, both while a fetched list shows one running and while our own
  // start or resume request is still pending (the list does not know yet).
  const running = busy || executions.some((e) => e.status === 'running')
  useEffect(() => {
    if (!running) return
    const id = setInterval(() => {
      refresh().catch((e: Error) => setNotice(e.message))
    }, 2000)
    return () => clearInterval(id)
  }, [running, refresh])

  // act runs one request against the API, then shows its result.
  const act = async (f: () => Promise<Execution>) => {
    setBusy(true)
    setNotice(null)
    try {
      const e = await f()
      await refresh()
      setSelected(e.id)
    } catch (e) {
      setNotice((e as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const shown = executions.find((e) => e.id === selected) ?? null
  return (
    <div className="split">
      <ExecutionList executions={executions} agents={agents} selected={selected} onSelect={setSelected} />
      <div className="stack">
        {notice && <div className="notice notice-error">{notice}</div>}
        <RunForm
          agents={agents}
          agentId={agentId}
          input={input}
          description={description}
          running={busy}
          onAgentChange={setAgentId}
          onInputChange={setInput}
          onDescriptionChange={setDescription}
          onRun={() => act(() => deps.start(agentId, input, description))}
        />
        {shown && (
          <ExecutionDetail
            execution={shown}
            agentName={agents.find((a) => a.id === shown.agentId)?.name ?? shown.agentId}
            busy={busy}
            onDecide={(approve, reason) => act(() => deps.decide(shown.id, shown.wait?.key ?? '', approve, reason))}
            onAnswer={(text) => act(() => deps.answer(shown.id, shown.wait?.key ?? '', text))}
            onCancel={(reason) => act(() => deps.cancel(shown.id, reason))}
            onRetry={(step) => act(() => deps.retry(shown.id, step))}
          />
        )}
      </div>
    </div>
  )
}
