import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { AttemptHistoryItem } from '../api/schemas'
import { formatDate } from '../lib/format'
import { renderApp } from './app'
import { historyItem, server } from './server'

const topicQuiz = historyItem({
  id: 'a2',
  scope: 'topic',
  chapterId: undefined,
  chapterTitle: undefined,
  chapterCount: 2,
  count: 10,
  scorePct: 85,
  submittedAt: '2026-11-21T10:05:00Z',
})
const chapterQuiz = historyItem({ id: 'a1', count: 5, scorePct: 40 })

function serveHistory(items: AttemptHistoryItem[]) {
  server.use(http.get('/api/attempts', () => HttpResponse.json(items)))
}

/** The History rows, each a link to its Results. */
async function rows() {
  return within(await screen.findByRole('list', { name: 'Attempts' })).getAllByRole('link')
}

describe('history page', () => {
  it('lists each attempt as a row that opens its Results', async () => {
    serveHistory([topicQuiz, chapterQuiz])
    renderApp('/history')
    const [topic, chapter] = await rows()
    if (topic === undefined || chapter === undefined) throw new Error('missing rows')

    expect(within(topic).getByText('All of Biology')).toBeInTheDocument()
    expect(topic).toHaveTextContent(
      `2 chapters · 10 questions · ${formatDate('2026-11-21T10:05:00Z')}`,
    )
    expect(within(topic).getByText('85%')).toBeInTheDocument()
    expect(topic).toHaveAttribute('href', '/history/a2')

    expect(within(chapter).getByText('Cells')).toBeInTheDocument()
    expect(chapter).toHaveTextContent(
      `Biology · 5 questions · ${formatDate('2026-11-20T10:05:00Z')}`,
    )
    expect(within(chapter).getByText('40%')).toBeInTheDocument()
    expect(chapter).toHaveAttribute('href', '/history/a1')
    expect(screen.queryByRole('link', { name: 'Review' })).not.toBeInTheDocument()
  })
  it('filters to topic or chapter quizzes', async () => {
    serveHistory([topicQuiz, chapterQuiz])
    const u = userEvent.setup()
    renderApp('/history')
    const titles = async () => (await rows()).map((r) => r.querySelector('p')?.textContent ?? '')
    const chip = (name: string) => screen.getByRole('button', { name })

    expect(await titles()).toEqual(['All of Biology', 'Cells'])
    expect(chip('All')).toHaveAttribute('aria-pressed', 'true')

    await u.click(chip('Topic quizzes'))
    expect(await titles()).toEqual(['All of Biology'])
    expect(chip('Topic quizzes')).toHaveAttribute('aria-pressed', 'true')
    expect(chip('All')).toHaveAttribute('aria-pressed', 'false')

    await u.click(chip('Chapter quizzes'))
    expect(await titles()).toEqual(['Cells'])

    await u.click(chip('All'))
    expect(await titles()).toEqual(['All of Biology', 'Cells'])
  })
  it('marks a quiz whose chapter or topic was deleted, keeping its score', async () => {
    serveHistory([
      { ...topicQuiz, scopeDeleted: true },
      { ...chapterQuiz, scopeDeleted: true },
    ])
    renderApp('/history')
    const [topic, chapter] = await rows()
    if (topic === undefined || chapter === undefined) throw new Error('missing rows')
    expect(topic.querySelector('p')).toHaveTextContent(/^All of Biology \(deleted\)$/)
    expect(within(topic).getByText('85%')).toBeInTheDocument()
    expect(chapter.querySelector('p')).toHaveTextContent(/^Cells \(deleted\)$/)
    expect(within(chapter).getByText('40%')).toBeInTheDocument()
    expect(chapter).toHaveAttribute('href', '/history/a1')
  })
})
