import { screen, within } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Dashboard } from '../api/schemas'
import { renderApp } from './app'
import { activityAttempt, activityUpload, dashboard, server } from './server'

function serveDashboard(d: Dashboard) {
  server.use(http.get('/api/dashboard', () => HttpResponse.json(d)))
}

/** A local time on a day in November 2026. */
const at = (day: number, hour: number) => new Date(2026, 10, day, hour).toISOString()

beforeEach(() => {
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(2026, 10, 27, 12)) // Friday 27 November, noon
})
afterEach(() => {
  vi.useRealTimers()
})

describe('dashboard', () => {
  it('shows one hero card leading to Upload when nothing is uploaded', async () => {
    serveDashboard(dashboard({ chapterCount: 0, topics: [], activity: [] }))
    renderApp('/')
    const main = within(await screen.findByRole('main'))

    expect(await main.findByRole('heading', { name: 'Turn a PDF into a quiz' })).toBeInTheDocument()
    expect(main.getByRole('link', { name: 'Upload a PDF' })).toHaveAttribute('href', '/upload')
    expect(main.queryByText(/lecture/i)).toBeNull()
    expect(main.queryByText('Average score')).toBeNull()
    expect(main.queryByRole('link', { name: 'Start a quiz' })).toBeNull()
  })

  it('shows the stat tiles, the chapters in progress and each topic', async () => {
    serveDashboard(
      dashboard({
        generatingCount: 1,
        failedCount: 2,
        stats: {
          averageScorePct: 66.67,
          attemptCount: 3,
          quizzesThisWeek: 2,
          questionsAnswered: 20,
          chaptersQuizzed: 1,
          readyChapters: 4,
        },
        topics: [
          {
            id: 't2',
            name: 'Algebra',
            readyChapters: 0,
            chaptersQuizzed: 0,
            averageLastScorePct: null,
          },
          {
            id: 't1',
            name: 'Biology',
            readyChapters: 4,
            chaptersQuizzed: 1,
            averageLastScorePct: 50,
          },
        ],
      }),
    )
    renderApp('/')
    const main = within(await screen.findByRole('main'))

    expect(await main.findByRole('heading', { level: 1, name: 'Dashboard' })).toBeInTheDocument()
    expect(main.getByText('1 generating')).toBeInTheDocument()
    expect(main.getByText('2 failed')).toBeInTheDocument()
    expect(main.getByRole('link', { name: 'Start a quiz' })).toHaveAttribute('href', '/quizzes')

    const tile = (label: string) => main.getByText(label).parentElement
    expect(tile('Average score')).toHaveTextContent('67%')
    expect(tile('Quizzes this week')).toHaveTextContent('2')
    expect(tile('Questions answered')).toHaveTextContent('20')
    expect(tile('Chapters quizzed')).toHaveTextContent('1 / 4')

    const [algebra, biology] = within(main.getByRole('list', { name: 'By topic' })).getAllByRole(
      'listitem',
    )
    if (algebra === undefined || biology === undefined) throw new Error('missing topic rows')
    expect(within(algebra).getByRole('link', { name: /Algebra/ })).toHaveAttribute(
      'href',
      '/topics/t2',
    )
    expect(algebra).toHaveTextContent('0 of 0 chapters quizzed')
    expect(within(algebra).getByText('Nothing ready')).toBeInTheDocument()
    expect(within(algebra).queryByRole('link', { name: 'Quiz topic' })).toBeNull()

    expect(biology).toHaveTextContent('1 of 4 chapters quizzed')
    expect(biology).toHaveTextContent('50%')
    expect(within(biology).getByRole('link', { name: 'Quiz topic' })).toHaveAttribute(
      'href',
      '/quizzes?topicId=t1',
    )
  })

  it('says "1 of 1 chapter quizzed" for a one-chapter topic', async () => {
    serveDashboard(
      dashboard({
        topics: [
          {
            id: 't1',
            name: 'Biology',
            readyChapters: 1,
            chaptersQuizzed: 1,
            averageLastScorePct: 80,
          },
        ],
      }),
    )
    renderApp('/')
    const list = await screen.findByRole('list', { name: 'By topic' })
    expect(list).toHaveTextContent('1 of 1 chapter quizzed')
  })

  it('offers no quiz while nothing is ready, and says no quizzes yet', async () => {
    serveDashboard(
      dashboard({
        failedCount: 1,
        stats: {
          averageScorePct: null,
          attemptCount: 0,
          quizzesThisWeek: 0,
          questionsAnswered: 0,
          chaptersQuizzed: 0,
          readyChapters: 0,
        },
        topics: [
          {
            id: 't1',
            name: 'Biology',
            readyChapters: 0,
            chaptersQuizzed: 0,
            averageLastScorePct: null,
          },
        ],
      }),
    )
    renderApp('/')
    const main = within(await screen.findByRole('main'))
    expect(await main.findByText('Average score')).toBeInTheDocument()
    expect(main.queryByRole('link', { name: 'Start a quiz' })).toBeNull()
    expect(main.getByText('Average score').parentElement).toHaveTextContent('No quizzes yet')
    expect(main.queryByText(/over 0/)).toBeNull()
  })

  it('groups the activity by day, newest first', async () => {
    serveDashboard(
      dashboard({
        activity: [
          activityAttempt({ id: 'a2', at: at(27, 10), scorePct: 90 }),
          activityAttempt({
            id: 'a1',
            at: at(27, 9),
            scope: 'topic',
            chapterId: undefined,
            chapterTitle: '',
            scorePct: 40,
            count: 10,
          }),
          activityUpload({ id: 'c3', at: at(26, 18), chapterTitle: 'Genes', status: 'pending' }),
          activityUpload({ id: 'c2', at: at(23, 8), chapterTitle: 'Broken', status: 'failed' }),
          activityUpload({ id: 'c1', at: at(23, 7), chapterTitle: 'Cells', questionCount: 10 }),
        ],
      }),
    )
    renderApp('/')
    const feed = within(await screen.findByRole('region', { name: 'Activity' }))

    const days = feed.getAllByRole('list')
    expect(days.map((d) => d.getAttribute('aria-label'))).toEqual([
      'Today',
      'Yesterday',
      expect.stringMatching(/Monday.*23|23.*Monday/),
    ])
    const [today, yesterday, monday] = days.map((d) => within(d).getAllByRole('link'))
    if (today === undefined || yesterday === undefined || monday === undefined)
      throw new Error('missing days')

    expect(today.map((r) => r.getAttribute('href'))).toEqual(['/history/a2', '/history/a1'])
    expect(today[0]).toHaveTextContent('Quizzed Cells')
    expect(today[0]).toHaveTextContent('90%')
    expect(today[1]).toHaveTextContent('Quizzed all of Biology')
    expect(today[1]).toHaveTextContent('40%')

    expect(yesterday[0]).toHaveAttribute('href', '/chapters/c3')
    expect(yesterday[0]).toHaveTextContent('Uploaded Genes')
    expect(yesterday[0]).toHaveTextContent('generating')

    expect(monday.map((r) => r.getAttribute('href'))).toEqual(['/chapters/c2', '/chapters/c1'])
    expect(monday[0]).toHaveTextContent('Uploaded Broken')
    expect(monday[0]).toHaveTextContent('failed')
    expect(monday[1]).toHaveTextContent('Uploaded Cells')
    expect(monday[1]).toHaveTextContent('10 questions')
  })
})
