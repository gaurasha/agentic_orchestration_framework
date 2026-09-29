import type { Http } from '../../lib/http'
import type { AgentsDeps } from './deps'
import type { Agent, ToolOption } from './types'

export function httpAgents(http: Http): AgentsDeps {
  return {
    list: () => http<Agent[]>('GET', '/v1/agents'),
    create: (def) => http<Agent>('POST', '/v1/agents', def),
    update: (id, def) => http<Agent>('PUT', `/v1/agents/${encodeURIComponent(id)}`, def),
    versions: (id) => http<Agent[]>('GET', `/v1/agents/${encodeURIComponent(id)}/versions`),
    tools: () => http<ToolOption[]>('GET', '/v1/tools'),
  }
}
