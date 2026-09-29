import type { Http } from '../../lib/http'

export interface Session {
  readonly tenant: string
  readonly token: string
}

const KEY = 'agent-orchestration.session'

export async function signIn(http: Http, tenant: string): Promise<Session> {
  const res = await http<{ token: string; tenant: string }>('POST', '/v1/token', { tenant })
  const session = { tenant: res.tenant, token: res.token }
  try {
    sessionStorage.setItem(KEY, JSON.stringify(session))
  } catch {
    // storage unavailable: the session lasts until the page reloads
  }
  return session
}

export function savedSession(): Session | null {
  try {
    const raw = sessionStorage.getItem(KEY)
    return raw ? (JSON.parse(raw) as Session) : null
  } catch {
    return null
  }
}

export function forgetSession(): void {
  try {
    sessionStorage.removeItem(KEY)
  } catch {
    // nothing stored
  }
}
