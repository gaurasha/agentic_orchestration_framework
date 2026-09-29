import { useCallback, useEffect, useState } from 'react'
import { useToolsDeps } from './deps'
import { ToolEditor } from './ToolEditor'
import { ToolList } from './ToolList'
import { emptyHTTPTool, type Tool } from './types'

type Editing = { readonly tool: Tool; readonly isNew: boolean; readonly params: string } | null

export interface ToolsPageProps {
  // refreshKey changes when credentials change, so the credential list reloads.
  readonly refreshKey: number
}

// ToolsPage holds the page's state; the components it renders are pure.
export function ToolsPage({ refreshKey }: ToolsPageProps) {
  const deps = useToolsDeps()
  const [tools, setTools] = useState<readonly Tool[]>([])
  const [refs, setRefs] = useState<readonly string[]>([])
  const [editing, setEditing] = useState<Editing>(null)
  const [saving, setSaving] = useState(false)
  const [notice, setNotice] = useState<{ readonly ok: boolean; readonly text: string } | null>(null)

  const refresh = useCallback(async () => {
    const [ts, rs] = await Promise.all([deps.list(), deps.credentialRefs()])
    setTools(ts)
    setRefs(rs)
  }, [deps])

  useEffect(() => {
    refresh().catch((e: Error) => setNotice({ ok: false, text: e.message }))
  }, [refresh, refreshKey])

  const edit = (tool: Tool, isNew: boolean) => {
    setNotice(null)
    setEditing({ tool, isNew, params: tool.params ? JSON.stringify(tool.params, null, 2) : '' })
  }

  const save = async () => {
    if (!editing) return
    let params: unknown
    try {
      params = editing.params.trim() ? JSON.parse(editing.params) : undefined
    } catch {
      setNotice({ ok: false, text: 'Arguments must be valid JSON.' })
      return
    }
    setSaving(true)
    try {
      const t = await deps.put({ ...editing.tool, params })
      setNotice({ ok: true, text: `Saved ${t.name}.` })
      setEditing(null)
      await refresh()
    } catch (e) {
      setNotice({ ok: false, text: (e as Error).message })
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="stack">
      {notice && <div className={`notice ${notice.ok ? 'notice-ok' : 'notice-error'}`}>{notice.text}</div>}
      <ToolList
        tools={tools}
        selected={editing && !editing.isNew ? editing.tool.name : null}
        onSelect={(name) => {
          const t = tools.find((x) => x.name === name)
          if (t) edit(t, false)
        }}
        onNew={() => edit(emptyHTTPTool, true)}
      />
      {editing && (
        <ToolEditor
          key={editing.isNew ? 'new' : editing.tool.name} // a fresh editor per tool, so no field keeps another tool's text
          tool={editing.tool}
          isNew={editing.isNew}
          params={editing.params}
          credentialRefs={refs}
          saving={saving}
          onChange={(tool) => setEditing({ ...editing, tool })}
          onParamsChange={(params) => setEditing({ ...editing, params })}
          onSave={save}
          onCancel={() => setEditing(null)}
        />
      )}
    </div>
  )
}
