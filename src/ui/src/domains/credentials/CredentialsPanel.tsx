import { useCallback, useEffect, useRef, useState } from 'react'
import { useCredentialsDeps } from './deps'

export interface CredentialsPanelProps {
  readonly onStored: () => void
}

// CredentialsPanel lists stored credentials by name, stores new values and
// replaces existing ones. A value is sent once and never shown again.
export function CredentialsPanel({ onStored }: CredentialsPanelProps) {
  const deps = useCredentialsDeps()
  const [refs, setRefs] = useState<readonly string[]>([])
  const [ref, setRef] = useState('')
  const [value, setValue] = useState('')
  const [notice, setNotice] = useState<{ readonly ok: boolean; readonly text: string } | null>(null)
  const valueInput = useRef<HTMLInputElement>(null)
  const replacing = refs.includes(ref.trim())

  const refresh = useCallback(() => deps.refs().then(setRefs), [deps])
  useEffect(() => {
    refresh().catch((e: Error) => setNotice({ ok: false, text: e.message }))
  }, [refresh])

  const store = async () => {
    const name = ref.trim()
    const replaced = refs.includes(name)
    try {
      await deps.store(name, value)
      setNotice({ ok: true, text: replaced ? `Replaced ${name}. Tools use the new value from their next call.` : `Stored ${name}.` })
      setRef('')
      setValue('')
      await refresh()
      onStored()
    } catch (e) {
      setNotice({ ok: false, text: (e as Error).message })
    }
  }

  return (
    <section className="card stack">
      <h2>Credentials</h2>
      <p className="muted" style={{ margin: 0 }}>Kept in memory. Values are write-only: a stored one can be replaced, never read.</p>
      {refs.length === 0 ? (
        <p className="muted">None stored.</p>
      ) : (
        <ul className="list">
          {refs.map((r) => (
            <li key={r} className="row-between" style={{ padding: '4px 12px' }}>
              <code>{r}</code>
              <span className="row">
                <span className="muted">••••••••</span>
                <button
                  type="button"
                  className="btn btn-small"
                  onClick={() => {
                    setNotice(null)
                    setRef(r)
                    setValue('')
                    valueInput.current?.focus()
                  }}
                >
                  Replace
                </button>
              </span>
            </li>
          ))}
        </ul>
      )}
      <form
        className="stack"
        onSubmit={(e) => {
          e.preventDefault()
          void store()
        }}
      >
        <label>
          Name
          <input value={ref} onChange={(e) => setRef(e.target.value)} placeholder="weather_key" required />
        </label>
        <label>
          Value
          <input ref={valueInput} type="password" autoComplete="off" value={value} onChange={(e) => setValue(e.target.value)} required />
        </label>
        {notice && <div className={`notice ${notice.ok ? 'notice-ok' : 'notice-error'}`}>{notice.text}</div>}
        <button className="btn" type="submit">{replacing ? `Replace ${ref.trim()}` : 'Store credential'}</button>
      </form>
    </section>
  )
}
