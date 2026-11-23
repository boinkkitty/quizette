import type { z } from 'zod'

import { errorBodySchema } from './schemas'

/** Error returned by the API (or a network/contract failure). */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly problems: readonly string[]

  constructor(status: number, code: string, message: string, problems: readonly string[] = []) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.problems = problems
  }
}

type Method = 'GET' | 'POST' | 'PATCH' | 'DELETE'

interface RequestOptions {
  method?: Method
  json?: unknown
  form?: FormData
  signal?: AbortSignal
}

/** Header the API requires on every state-changing request (CSRF guard). */
export const csrfHeader = { 'X-Requested-With': 'quizette' } as const

async function send(
  path: string,
  { method = 'GET', json, form, signal }: RequestOptions,
): Promise<Response> {
  const headers = new Headers({ Accept: 'application/json' })
  // The session cookie rides on every request, so every mutation carries the CSRF header.
  if (method !== 'GET') headers.set('X-Requested-With', csrfHeader['X-Requested-With'])
  let body: BodyInit | null = null
  if (form !== undefined) {
    body = form
  } else if (json !== undefined) {
    headers.set('Content-Type', 'application/json')
    body = JSON.stringify(json)
  }
  let res: Response
  try {
    res = await fetch(`/api${path}`, {
      method,
      headers,
      body,
      credentials: 'same-origin',
      signal: signal ?? null,
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(
      0,
      'network',
      'Could not reach the server. Check your connection and try again.',
    )
  }
  if (!res.ok) {
    const parsed = errorBodySchema.safeParse(await res.json().catch(() => null))
    if (parsed.success) {
      throw new ApiError(
        res.status,
        parsed.data.error,
        parsed.data.message,
        parsed.data.problems ?? [],
      )
    }
    throw new ApiError(res.status, 'http', `Request failed (${String(res.status)})`)
  }
  return res
}

/** Performs a request and validates the JSON response against schema. */
export async function request<S extends z.ZodType>(
  schema: S,
  path: string,
  opts: RequestOptions = {},
): Promise<z.infer<S>> {
  const res = await send(path, opts)
  const parsed = schema.safeParse(await res.json())
  if (!parsed.success) {
    console.error('API contract violation', path, parsed.error.issues)
    throw new ApiError(res.status, 'contract', 'The server sent an unexpected response.')
  }
  return parsed.data
}

/** Performs a request that returns no body (204). */
export async function requestVoid(path: string, opts: RequestOptions = {}): Promise<void> {
  await send(path, opts)
}

export function isApiError(err: unknown): err is ApiError {
  return err instanceof ApiError
}
