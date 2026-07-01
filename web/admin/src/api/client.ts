import type { ApiError } from './types'

const API_KEY = import.meta.env.VITE_API_KEY as string | undefined

export class ApiClientError extends Error {
  status: number
  code?: string

  constructor(message: string, status: number, code?: string) {
    super(message)
    this.name = 'ApiClientError'
    this.status = status
    this.code = code
  }
}

type QueryValue = string | number | undefined | null

function buildQuery(params: Record<string, QueryValue>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      search.set(key, String(value))
    }
  }
  const q = search.toString()
  return q ? `?${q}` : ''
}

async function parseError(res: Response): Promise<ApiClientError> {
  let body: ApiError = { error: res.statusText }
  try {
    body = (await res.json()) as ApiError
  } catch {
    /* empty body */
  }
  return new ApiClientError(body.error || 'Request failed', res.status, body.code)
}

export async function apiGet<T>(path: string, params?: Record<string, QueryValue>): Promise<T> {
  const url = `${path}${params ? buildQuery(params) : ''}`
  const res = await fetch(url, {
    headers: { Accept: 'application/json' },
  })
  if (!res.ok) {
    throw await parseError(res)
  }
  return res.json() as Promise<T>
}

export async function apiPost<T>(path: string, idempotencyKey?: string): Promise<T> {
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'Content-Type': 'application/json',
  }
  if (API_KEY) {
    headers['X-API-Key'] = API_KEY
  }
  if (idempotencyKey) {
    headers['Idempotency-Key'] = idempotencyKey
  }
  const res = await fetch(path, {
    method: 'POST',
    headers,
    body: '{}',
  })
  if (!res.ok) {
    throw await parseError(res)
  }
  return res.json() as Promise<T>
}

export function newIdempotencyKey(prefix: string): string {
  return `${prefix}_${crypto.randomUUID()}`
}
