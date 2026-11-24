import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'

import { server } from '../test/server'
import { ServerWakeBanner } from './ServerWakeBanner'

const WAKE_URL = 'https://api.render.example/healthz'

/** Render behind the Worker: 502 + wake URL while asleep; one direct request wakes it. */
function sleepingRender() {
  const state = { awake: false, wakeRequests: 0 }
  server.use(
    http.get('/api/auth/providers', () =>
      state.awake
        ? HttpResponse.json({ providers: [] })
        : new HttpResponse(null, { status: 502, headers: { 'X-Wake-Url': WAKE_URL } }),
    ),
    http.get(WAKE_URL, () => {
      state.wakeRequests++
      state.awake = true
      return new HttpResponse('ok')
    }),
  )
  return state
}

describe('ServerWakeBanner', () => {
  it('wakes Render directly, then reloads the failed routes', async () => {
    const state = sleepingRender()
    const onAwake = vi.fn<() => Promise<void>>(() => Promise.resolve())
    render(<ServerWakeBanner onAwake={onAwake} delayMs={0} intervalMs={300} />)

    expect(await screen.findByText(/Waking up the server/)).toBeInTheDocument()
    await waitFor(() => expect(onAwake).toHaveBeenCalledOnce())
    expect(state.wakeRequests).toBe(1)
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('stays hidden and sends no wake request when the API is already up', async () => {
    const state = sleepingRender()
    state.awake = true
    const onAwake = vi.fn<() => Promise<void>>(() => Promise.resolve())
    render(<ServerWakeBanner onAwake={onAwake} delayMs={50} intervalMs={10} />)

    await new Promise((r) => setTimeout(r, 100))
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(onAwake).not.toHaveBeenCalled()
    expect(state.wakeRequests).toBe(0)
  })

  it('offers a retry instead of spinning forever', async () => {
    let up = false
    server.use(
      http.get('/api/auth/providers', () =>
        up ? HttpResponse.json({ providers: [] }) : new HttpResponse(null, { status: 502 }),
      ),
    )
    const onAwake = vi.fn<() => Promise<void>>(() => Promise.resolve())
    render(<ServerWakeBanner onAwake={onAwake} delayMs={0} intervalMs={10} maxWaitMs={50} />)

    const retry = await screen.findByRole('button', { name: 'Try again' })
    expect(screen.getByText(/taking longer than usual/)).toBeInTheDocument()
    up = true
    await userEvent.setup().click(retry)
    await waitFor(() => expect(screen.queryByRole('status')).not.toBeInTheDocument())
  })
})
