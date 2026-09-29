import { useCallback, useEffect, useState } from 'react'
import { AgentEditor } from './AgentEditor'
import { AgentList } from './AgentList'
import { AgentVersions } from './AgentVersions'
import { useAgentsDeps } from './deps'
import { definitionOf, emptyDefinition, type Agent, type Definition, type ToolOption } from './types'

type Mode =
  | { readonly kind: 'none' }
  | { readonly kind: 'view'; readonly id: string; readonly version: number }
  | { readonly kind: 'edit'; readonly id: string | null; readonly def: Definition }

// AgentsPage holds the page's state; the components it renders are pure.
export function AgentsPage() {
  const deps = useAgentsDeps()
  const [agents, setAgents] = useState<readonly Agent[]>([])
  const [tools, setTools] = useState<readonly ToolOption[]>([])
  const [versions, setVersions] = useState<readonly Agent[]>([])
  const [mode, setMode] = useState<Mode>({ kind: 'none' })
  const [saving, setSaving] = useState(false)
  const [notice, setNotice] = useState<{ readonly ok: boolean; readonly text: string } | null>(null)

  const refresh = useCallback(async () => {
    const [as, ts] = await Promise.all([deps.list(), deps.tools()])
    setAgents(as)
    setTools(ts)
  }, [deps])

  useEffect(() => {
    refresh().catch((e: Error) => setNotice({ ok: false, text: e.message }))
  }, [refresh])

  const show = useCallback(
    async (id: string, version?: number) => {
      const vs = await deps.versions(id)
      setVersions(vs)
      setMode({ kind: 'view', id, version: version ?? vs[vs.length - 1]?.version ?? 1 })
    },
    [deps],
  )

  const save = async () => {
    if (mode.kind !== 'edit') return
    setSaving(true)
    setNotice(null)
    try {
      const before = mode.id ? agents.find((a) => a.id === mode.id)?.version : undefined
      const a = mode.id ? await deps.update(mode.id, mode.def) : await deps.create(mode.def)
      setNotice({
        ok: true,
        text: before === a.version ? `No changes: still v${a.version}.` : `Saved ${a.name} v${a.version}.`,
      })
      await refresh()
      await show(a.id, a.version)
    } catch (e) {
      setNotice({ ok: false, text: (e as Error).message })
    } finally {
      setSaving(false)
    }
  }

  const selected = mode.kind === 'none' ? null : mode.id
  return (
    <div className="split">
      <AgentList
        agents={agents}
        selected={selected}
        onSelect={(id) => {
          setNotice(null)
          show(id).catch((e: Error) => setNotice({ ok: false, text: e.message }))
        }}
        onNew={() => {
          setNotice(null)
          setMode({ kind: 'edit', id: null, def: emptyDefinition })
        }}
      />
      <div className="stack">
        {notice && <div className={`notice ${notice.ok ? 'notice-ok' : 'notice-error'}`}>{notice.text}</div>}
        {mode.kind === 'none' && <div className="card empty">Select an agent, or create one.</div>}
        {mode.kind === 'view' && (
          <AgentVersions
            versions={versions}
            tools={tools}
            shown={mode.version}
            onShow={(version) => setMode({ ...mode, version })}
            onEdit={() => {
              const latest = versions[versions.length - 1]
              if (latest) setMode({ kind: 'edit', id: latest.id, def: definitionOf(latest) })
            }}
          />
        )}
        {mode.kind === 'edit' && (
          <AgentEditor
            key={mode.id ?? 'new'}
            title={mode.id ? 'Edit agent: saving makes a new version' : 'New agent'}
            def={mode.def}
            tools={tools}
            saving={saving}
            onChange={(def) => setMode({ ...mode, def })}
            onSave={save}
            onCancel={() => (mode.id ? show(mode.id) : setMode({ kind: 'none' }))}
          />
        )}
      </div>
    </div>
  )
}
