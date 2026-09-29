import { useCallback, useMemo, useState } from 'react'
import { createHttp } from './lib/http'
import { AgentsProvider } from './domains/agents/deps'
import { httpAgents } from './domains/agents/http'
import { AgentsPage } from './domains/agents/AgentsPage'
import { ToolsProvider } from './domains/tools/deps'
import { httpTools } from './domains/tools/http'
import { ToolsPage } from './domains/tools/ToolsPage'
import { CredentialsProvider } from './domains/credentials/deps'
import { httpCredentials } from './domains/credentials/http'
import { CredentialsPanel } from './domains/credentials/CredentialsPanel'
import { ExecutionsProvider } from './domains/executions/deps'
import { httpExecutions } from './domains/executions/http'
import { ExecutionsPage } from './domains/executions/ExecutionsPage'
import { SignIn } from './domains/session/SignIn'
import { forgetSession, savedSession, signIn, type Session } from './domains/session/session'

type Tab = 'agents' | 'tools' | 'executions'

// App wires each domain to the HTTP API and composes the pages; it is the
// only place that knows more than one domain.
export function App() {
  const [session, setSession] = useState<Session | null>(savedSession)
  const [error, setError] = useState<string | null>(null)
  const [tab, setTab] = useState<Tab>('agents')
  const [credentialsVersion, setCredentialsVersion] = useState(0)

  const signOut = useCallback(() => {
    forgetSession()
    setSession(null)
  }, [])
  // A 401 with a token means the server no longer knows it: its signing
  // key is per process, so every restart signs every tab out.
  const expired = useCallback(() => {
    signOut()
    setError('Signed out: the server was restarted and no longer knows this session. Sign in again.')
  }, [signOut])
  const http = useMemo(() => createHttp(session?.token ?? null, expired), [session, expired])
  const deps = useMemo(
    () => ({
      agents: httpAgents(http),
      tools: httpTools(http),
      credentials: httpCredentials(http),
      executions: httpExecutions(http),
    }),
    [http],
  )

  if (!session) {
    return (
      <SignIn
        error={error}
        onSignIn={(tenant) => {
          setError(null)
          signIn(http, tenant).then(setSession, (e: Error) => setError(e.message))
        }}
      />
    )
  }

  return (
    <>
      <header className="app-header">
        <h1>Agent Orchestration</h1>
        <nav>
          <button className="tab" aria-current={tab === 'agents' ? 'page' : undefined} onClick={() => setTab('agents')}>
            Agents
          </button>
          <button className="tab" aria-current={tab === 'tools' ? 'page' : undefined} onClick={() => setTab('tools')}>
            Tools
          </button>
          <button className="tab" aria-current={tab === 'executions' ? 'page' : undefined} onClick={() => setTab('executions')}>
            Executions
          </button>
        </nav>
        <span className="badge badge-accent">{session.tenant}</span>
        <button className="btn btn-small" onClick={signOut}>Sign out</button>
      </header>
      <main>
        {tab === 'agents' && (
          <AgentsProvider deps={deps.agents}>
            <AgentsPage />
          </AgentsProvider>
        )}
        {tab === 'tools' && (
          <div className="split split-reverse">
            <ToolsProvider deps={deps.tools}>
              <ToolsPage refreshKey={credentialsVersion} />
            </ToolsProvider>
            <CredentialsProvider deps={deps.credentials}>
              <CredentialsPanel onStored={() => setCredentialsVersion((n) => n + 1)} />
            </CredentialsProvider>
          </div>
        )}
        {tab === 'executions' && (
          <ExecutionsProvider deps={deps.executions}>
            <ExecutionsPage />
          </ExecutionsProvider>
        )}
      </main>
    </>
  )
}
