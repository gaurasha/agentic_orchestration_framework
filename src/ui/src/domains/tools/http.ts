import type { Http } from '../../lib/http'
import type { ToolsDeps } from './deps'
import type { Tool } from './types'

export function httpTools(http: Http): ToolsDeps {
  return {
    list: () => http<Tool[]>('GET', '/v1/tools'),
    put: (t) => http<Tool>('PUT', `/v1/tools/${encodeURIComponent(t.name)}`, t),
    credentialRefs: () => http<string[]>('GET', '/v1/credentials'),
  }
}
