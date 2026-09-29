// A fetch wrapper for the /v1 API: adds the bearer token and turns an
// error response into an ApiError carrying the server's message.

export class ApiError extends Error {
  constructor(readonly status: number, message: string) {
    super(message)
  }
}

export type Http = <T>(method: string, path: string, body?: unknown) => Promise<T>

export function createHttp(token: string | null, onUnauthorized: () => void): Http {
  return async <T,>(method: string, path: string, body?: unknown): Promise<T> => {
    const headers: Record<string, string> = {}
    if (token) headers.Authorization = `Bearer ${token}`
    if (body !== undefined) headers['Content-Type'] = 'application/json'
    const res = await fetch(path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) })
    if (res.status === 401 && token) onUnauthorized()
    const text = await res.text()
    const data: unknown = text ? JSON.parse(text) : undefined
    if (!res.ok) {
      const message = (data as { error?: string } | undefined)?.error ?? res.statusText
      throw new ApiError(res.status, message)
    }
    return data as T
  }
}
