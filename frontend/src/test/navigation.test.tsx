import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import { renderApp } from './app'
import { chapter, chapterSummary, historyItem, job, server, topic } from './server'

// Each page has one fixed parent, and its back link always goes there (issue #19).

describe('topics browser', () => {
  it('opens from the sidebar and lists the topics', async () => {
    const u = userEvent.setup()
    const { router } = renderApp('/history')
    server.use(http.get('/api/attempts', () => HttpResponse.json([])))
    const nav = await screen.findByRole('navigation', { name: 'Main' })
    await u.click(within(nav).getByRole('link', { name: 'Topics' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/topics'))
    expect(await screen.findByRole('heading', { name: 'Topics' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Biology/ })).toHaveAttribute('href', '/topics/t1')
    expect(within(nav).getByRole('link', { name: 'Topics' })).toHaveAttribute(
      'aria-current',
      'page',
    )
  })
})

describe('topic page', () => {
  /** Serves topic t1 (Biology) with a chapter in each status. */
  function serveTopic() {
    server.use(
      http.get('/api/topics/t1', () =>
        HttpResponse.json({
          topic: topic({ chapterCount: 3 }),
          chapters: [
            chapterSummary(),
            chapterSummary({ id: 'c2', title: 'Genetics', status: 'pending', questionCount: 0 }),
            chapterSummary({ id: 'c3', title: 'Ecology', status: 'failed', questionCount: 0 }),
          ],
        }),
      ),
    )
  }

  it('leads back to Topics, even when opened from History', async () => {
    serveTopic()
    server.use(
      http.get('/api/attempts', () =>
        HttpResponse.json([historyItem({ scope: 'topic', chapterId: undefined })]),
      ),
    )
    const u = userEvent.setup()
    const { router } = renderApp('/history')
    await u.click(await screen.findByRole('link', { name: 'All of Biology' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/topics/t1'))
    expect(await screen.findByRole('link', { name: '← Topics' })).toHaveAttribute('href', '/topics')
  })

  it('lists every chapter and offers Add chapter and Quiz this topic', async () => {
    serveTopic()
    renderApp('/topics/t1')
    const links = await screen.findAllByRole('link', { name: /Cells|Genetics|Ecology/ })
    expect(links.map((l) => l.getAttribute('href'))).toEqual([
      '/chapters/c1',
      '/chapters/c2',
      '/chapters/c3',
    ])
    expect(screen.getByRole('link', { name: 'Add chapter' })).toHaveAttribute(
      'href',
      '/upload?topicId=t1',
    )
    expect(screen.getByRole('link', { name: 'Quiz this topic' })).toHaveAttribute(
      'href',
      '/quizzes?topicId=t1',
    )
  })

  it('goes to Topics after deleting the topic', async () => {
    serveTopic()
    server.use(http.delete('/api/topics/t1', () => new HttpResponse(null, { status: 204 })))
    const u = userEvent.setup()
    const { router } = renderApp('/topics/t1')
    await u.click(await screen.findByRole('button', { name: 'Delete topic' }))
    await u.click(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Delete topic' }),
    )
    await waitFor(() => expect(router.state.location.pathname).toBe('/topics'))
  })
})

describe('chapter page', () => {
  function serveChapter() {
    server.use(
      http.get('/api/chapters/c1', () => HttpResponse.json(chapter())),
      http.get('/api/chapters/c1/attempts', () => HttpResponse.json([])),
    )
  }

  it('leads back to its topic, even from History, and only links out to quiz or view', async () => {
    serveChapter()
    server.use(http.get('/api/attempts', () => HttpResponse.json([historyItem()])))
    const u = userEvent.setup()
    const { router } = renderApp('/history')
    await u.click(await screen.findByRole('link', { name: 'Cells' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/chapters/c1'))

    expect(await screen.findByRole('link', { name: '← Biology' })).toHaveAttribute(
      'href',
      '/topics/t1',
    )
    expect(screen.getByRole('link', { name: 'Quiz this chapter' })).toHaveAttribute(
      'href',
      '/quizzes?chapterId=c1',
    )
    expect(screen.getByRole('link', { name: 'View questions' })).toHaveAttribute(
      'href',
      '/chapters/c1/questions',
    )
    // The picker on Quizzes is the only place to set up a quiz, and Questions the only bank viewer.
    expect(screen.queryByLabelText('Number of questions')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Start quiz' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /question bank/ })).not.toBeInTheDocument()
  })
})

describe('questions page', () => {
  it('leads back to its chapter, from the header and from Done', async () => {
    server.use(
      http.get('/api/chapters/c1', () => HttpResponse.json(chapter())),
      http.get('/api/chapters/c1/question-set', () =>
        HttpResponse.json({ id: 's1', chapterId: 'c1', questions: [] }),
      ),
    )
    renderApp('/chapters/c1/questions')
    expect(await screen.findByRole('link', { name: '← Cells' })).toHaveAttribute(
      'href',
      '/chapters/c1',
    )
  })
})

describe('upload page', () => {
  it('offers Quiz this chapter, Go to chapter and Upload another PDF once ready', async () => {
    server.use(
      http.post('/api/documents', () =>
        HttpResponse.json({ jobId: 'j1', chapterId: 'c1', topicId: 't1' }, { status: 202 }),
      ),
      http.get('/api/jobs/j1', () => HttpResponse.json(job({ status: 'succeeded' }))),
    )
    const u = userEvent.setup()
    renderApp('/upload')
    await u.upload(
      await screen.findByLabelText('PDF file'),
      new File(['%PDF-1.4'], 'cells.pdf', { type: 'application/pdf' }),
    )
    await u.click(screen.getByRole('button', { name: 'Generate questions' }))
    await screen.findByText('Your question bank is ready')

    const main = screen.getByRole('main')
    const actions = within(main)
      .getAllByRole('link')
      .concat(within(main).getAllByRole('button'))
      .map((el) => [el.textContent, el.getAttribute('href')])
    expect(actions).toEqual([
      ['Quiz this chapter', '/quizzes?chapterId=c1'],
      ['Go to chapter', '/chapters/c1'],
      ['Upload another PDF', null],
    ])
  })
})

describe('dashboard', () => {
  it('leaves uploading to the sidebar', async () => {
    renderApp('/')
    await screen.findByRole('heading', { name: 'Your topics' })
    expect(within(screen.getByRole('main')).queryByRole('link', { name: /Upload/ })).toBeNull()
  })
})

describe('empty states', () => {
  it.each([
    ['/topics', '/upload'],
    ['/quizzes', '/upload'],
    ['/history', '/upload'],
    ['/topics/t1', '/upload?topicId=t1'],
  ])('%s with nothing yet leads to Upload', async (path, href) => {
    server.use(
      http.get('/api/topics', () => HttpResponse.json([])),
      http.get('/api/chapters', () => HttpResponse.json([])),
      http.get('/api/attempts', () => HttpResponse.json([])),
      http.get('/api/topics/t1', () =>
        HttpResponse.json({ topic: topic({ chapterCount: 0 }), chapters: [] }),
      ),
    )
    renderApp(path)
    const empty = await screen.findByText(/^(No|Nothing to) .* yet$/)
    const link = within(empty.parentElement as HTMLElement).getByRole('link', { name: /Upload/ })
    expect(link).toHaveAttribute('href', href)
  })
})
