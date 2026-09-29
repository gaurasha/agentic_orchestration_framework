import { useState } from 'react'

export interface SignInProps {
  readonly error: string | null
  readonly onSignIn: (tenant: string) => void
}

// SignIn asks for a tenant; sign-in proper is out of scope.
export function SignIn({ error, onSignIn }: SignInProps) {
  const [tenant, setTenant] = useState('acme')
  return (
    <main>
      <form
        className="card stack"
        style={{ maxWidth: 360, margin: '80px auto' }}
        onSubmit={(e) => {
          e.preventDefault()
          onSignIn(tenant.trim())
        }}
      >
        <h1>Agent Orchestration</h1>
        <p className="muted">Pick a tenant. Everything you create is visible only to that tenant.</p>
        <label>
          Tenant
          <input value={tenant} onChange={(e) => setTenant(e.target.value)} autoFocus />
        </label>
        {error && <div className="notice notice-error">{error}</div>}
        <button className="btn btn-primary" type="submit">Sign in</button>
      </form>
    </main>
  )
}
