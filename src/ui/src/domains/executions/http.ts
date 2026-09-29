import type { Http } from '../../lib/http'
import type { ExecutionsDeps } from './deps'
import type { AgentOption, Execution } from './types'

export function httpExecutions(http: Http): ExecutionsDeps {
  const at = (id: string, action: string) => `/v1/executions/${encodeURIComponent(id)}/${action}`
  return {
    list: () => http<Execution[]>('GET', '/v1/executions'),
    start: (agentId, input, description) => http<Execution>('POST', '/v1/executions', { agentId, input, description }),
    decide: (id, key, approve, reason) => http<Execution>('POST', at(id, 'decide'), { key, approve, reason }),
    answer: (id, key, text) => http<Execution>('POST', at(id, 'answer'), { key, text }),
    cancel: (id, reason) => http<Execution>('POST', at(id, 'cancel'), { reason }),
    retry: (id, step) => http<Execution>('POST', at(id, `steps/${step}/retry`)),
    agents: () => http<AgentOption[]>('GET', '/v1/agents'),
  }
}
