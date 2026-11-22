import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'

import { renderWithQuery } from '../test/render'
import { job, server } from '../test/server'
import { JobProgress } from './JobProgress'

// JobProgress links to a route; stub Link to avoid needing a router here.
vi.mock('@tanstack/react-router', () => ({ Link: ({ children }: { children: unknown }) => children }))

describe('JobProgress', () => {
  it('polls until the job succeeds', async () => {
    const statuses = ['queued', 'running', 'succeeded'] as const
    let call = 0
    server.use(http.get('/api/jobs/j1', () => HttpResponse.json(job({ status: statuses[Math.min(call++, 2)] ?? 'succeeded' }))))
    renderWithQuery(<JobProgress jobId="j1" chapterId="c1" />)
    expect(await screen.findByText('Generating 10 questions')).toBeInTheDocument()
    expect(await screen.findByText('Your question bank is ready', {}, { timeout: 6000 })).toBeInTheDocument()
    expect(call).toBe(3)
  }, 10_000)

  it('shows the failure reason', async () => {
    server.use(http.get('/api/jobs/j1', () => HttpResponse.json(job({ status: 'failed', error: "we couldn't read any text from this PDF" }))))
    renderWithQuery(<JobProgress jobId="j1" chapterId="c1" />)
    expect(await screen.findByText("we couldn't read any text from this PDF")).toBeInTheDocument()
  })
})
