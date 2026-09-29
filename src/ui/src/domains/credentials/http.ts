import type { Http } from '../../lib/http'
import type { CredentialsDeps } from './deps'

export function httpCredentials(http: Http): CredentialsDeps {
  return {
    refs: () => http<string[]>('GET', '/v1/credentials'),
    store: (ref, value) => http<void>('PUT', `/v1/credentials/${encodeURIComponent(ref)}`, { value }),
  }
}
