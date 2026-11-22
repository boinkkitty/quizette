import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { z } from 'zod'

import { server, unauthorized } from '../test/server'
import { ApiError, request } from './client'

describe('request', () => {
  it('sends the CSRF header on every mutation, and never an Authorization header', async () => {
    const seen: Array<{ csrf: string | null; auth: string | null; credentials: string }> = []
    const record = ({ request: req }: { request: Request }) => {
      seen.push({ csrf: req.headers.get('X-Requested-With'), auth: req.headers.get('Authorization'), credentials: req.credentials })
      return HttpResponse.json({ ok: true })
    }
    server.use(http.all('/api/echo', record), http.post('/api/auth/echo', record))
    const schema = z.object({ ok: z.boolean() })
    await request(schema, '/echo')
    await request(schema, '/echo', { method: 'POST', json: {} })
    await request(schema, '/echo', { method: 'DELETE' })
    await request(schema, '/auth/echo', { method: 'POST', json: {} })
    expect(seen.map((r) => r.csrf)).toEqual([null, 'quizette', 'quizette', 'quizette'])
    expect(seen.every((r) => r.auth === null && r.credentials === 'same-origin')).toBe(true)
  })

  it('reports 401 without retrying', async () => {
    let calls = 0
    server.use(
      http.get('/api/echo', () => {
        calls++
        return unauthorized()
      }),
    )
    const err = await request(z.object({}), '/echo').catch((e: unknown) => e)
    expect(err).toMatchObject({ status: 401, code: 'unauthorized' })
    expect(calls).toBe(1)
  })

  it('maps error bodies to ApiError with problems', async () => {
    server.use(http.post('/api/fail', () => HttpResponse.json({ error: 'validation', message: 'bad count', problems: ['bad count'] }, { status: 400 })))
    const err = await request(z.object({}), '/fail', { method: 'POST', json: {} }).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 400, code: 'validation', message: 'bad count', problems: ['bad count'] })
  })

  it('rejects responses that violate the schema', async () => {
    server.use(http.get('/api/weird', () => HttpResponse.json({ id: 42 })))
    const err = await request(z.object({ id: z.string() }), '/weird').catch((e: unknown) => e)
    expect(err).toMatchObject({ code: 'contract' })
  })

  it('reports network failures', async () => {
    server.use(http.get('/api/down', () => HttpResponse.error()))
    const err = await request(z.object({}), '/down').catch((e: unknown) => e)
    expect(err).toMatchObject({ status: 0, code: 'network' })
  })
})
