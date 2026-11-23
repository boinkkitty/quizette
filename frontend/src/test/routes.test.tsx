import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { Attempt } from '../api/schemas'
import { renderApp } from './app'
import { attempt, chapterSummary, historyItem, server, unauthorized, user } from './server'

describe('routing', () => {
  it('redirects logged-out users to login, preserving the destination', async () => {
    server.use(http.get('/api/auth/me', unauthorized))
    const { router } = renderApp('/upload')
    expect(await screen.findByRole('heading', { name: 'Log in to Quizette' })).toBeInTheDocument()
    expect(router.state.location.search).toEqual({ redirect: '/upload' })
    expect(await screen.findByRole('link', { name: 'Continue with GitHub' })).toHaveAttribute('href', '/api/auth/github/start')
  })

  it('shows Google before GitHub whatever order the API lists them', async () => {
    server.use(
      http.get('/api/auth/me', unauthorized),
      http.get('/api/auth/providers', () => HttpResponse.json({ providers: ['github', 'google'] })),
    )
    renderApp('/login')
    const github = await screen.findByRole('link', { name: 'Continue with GitHub' })
    const links = screen.getAllByRole('link', { name: /^Continue with/ })
    expect(links.map((l) => l.textContent)).toEqual(['Continue with Google', 'Continue with GitHub'])
    expect(links[0]).toHaveAttribute('href', '/api/auth/google/start')
    expect(github).toHaveAttribute('href', '/api/auth/github/start')
  })

  it('logs in and lands on the requested page', async () => {
    let loggedIn = false
    server.use(
      http.get('/api/auth/me', () => (loggedIn ? HttpResponse.json(user) : unauthorized())),
      http.post('/api/auth/login', () => {
        loggedIn = true
        return HttpResponse.json(user)
      }),
    )
    const u = userEvent.setup()
    const { router } = renderApp('/login?redirect=%2F')
    await u.type(await screen.findByLabelText('Email'), 'ada@example.com')
    await u.type(screen.getByLabelText('Password'), 'correct horse')
    await u.click(screen.getByRole('button', { name: 'Log in' }))
    expect(await screen.findByRole('heading', { name: 'Your topics' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/')
  })

  it('shows login errors from the API', async () => {
    server.use(
      http.get('/api/auth/me', unauthorized),
      http.post('/api/auth/login', () => HttpResponse.json({ error: 'unauthorized', message: 'invalid email or password' }, { status: 401 })),
    )
    const u = userEvent.setup()
    renderApp('/login')
    await u.type(await screen.findByLabelText('Email'), 'ada@example.com')
    await u.type(screen.getByLabelText('Password'), 'wrong')
    await u.click(screen.getByRole('button', { name: 'Log in' }))
    expect(await screen.findByText('invalid email or password')).toBeInTheDocument()
  })

  it('takes a quiz and shows partial MRQ credit on the results page', async () => {
    const submitted: Attempt = attempt({
      status: 'submitted', totalScore: 2.5, scorePct: 50, submittedAt: '2026-11-20T10:05:00Z',
      questions: attempt().questions.map((q) => ({ ...q, correct: q.type === 'mrq' ? [0, 2] : [q.type === 'mcq' ? 1 : 0], explanation: 'Because.' })),
      answers: [
        { questionId: 'q1', selected: [1], points: 1, correct: true },
        { questionId: 'q2', selected: [1], points: 0, correct: false },
        { questionId: 'q3', selected: [0], points: 0.5, correct: false },
      ],
    })
    let current = attempt()
    let body: unknown
    server.use(
      http.get('/api/attempts/a1', () => HttpResponse.json(current)),
      http.post('/api/attempts/a1/submit', async ({ request }) => {
        body = await request.json()
        current = submitted
        return HttpResponse.json(submitted)
      }),
      http.get('/api/chapters/c1/attempts', () => HttpResponse.json([])),
    )
    const u = userEvent.setup()
    const { router } = renderApp('/attempts/a1')
    await u.click(await screen.findByRole('radio', { name: 'Mitochondria' }))
    await u.click(screen.getByRole('button', { name: 'Next' }))
    await u.click(screen.getByRole('radio', { name: 'False' }))
    await u.click(screen.getByRole('button', { name: 'Next' }))
    await u.click(screen.getByRole('checkbox', { name: 'Nucleus' }))
    await u.click(screen.getByRole('button', { name: 'Submit quiz' }))

    expect(await screen.findByTestId('score')).toHaveTextContent('2.5 / 5')
    expect(window.localStorage.getItem('quizette:quiz-progress:a1')).toBeNull() // saved progress cleared once submitted
    expect(router.state.location.pathname).toBe('/attempts/a1/results')
    expect(screen.getByText('0.5 / 1 point')).toBeInTheDocument()
    expect(body).toEqual({ answers: [{ questionId: 'q1', selected: [1] }, { questionId: 'q2', selected: [1] }, { questionId: 'q3', selected: [0] }] })
  })

  it('redirects a submitted attempt to its results', async () => {
    server.use(http.get('/api/attempts/a1', () => HttpResponse.json(attempt({ status: 'submitted', submittedAt: '2026-11-20T10:05:00Z' }))))
    const { router } = renderApp('/attempts/a1')
    await waitFor(() => expect(router.state.location.pathname).toBe('/attempts/a1/results'))
  })
})

describe('session', () => {
  it('restores the session from the cookie on load', async () => {
    renderApp('/')
    expect(await screen.findByRole('heading', { name: 'Your topics' })).toBeInTheDocument()
    expect(screen.getByText('Ada')).toBeInTheDocument()
  })

  it('signs out: ends the session on the server and shows the login page', async () => {
    let loggedOut = false
    server.use(
      http.get('/api/auth/me', () => (loggedOut ? unauthorized() : HttpResponse.json(user))),
      http.post('/api/auth/logout', ({ request }) => {
        expect(request.headers.get('X-Requested-With')).toBe('quizette')
        loggedOut = true
        return new HttpResponse(null, { status: 204 })
      }),
    )
    const u = userEvent.setup()
    renderApp('/')
    await u.click(await screen.findByRole('button', { name: 'Sign out' }))
    expect(await screen.findByRole('heading', { name: 'Log in to Quizette' })).toBeInTheDocument()
    expect(loggedOut).toBe(true)
  })

  it('sends the user to login, then back, when the session has expired', async () => {
    let expired = false
    server.use(
      http.get('/api/auth/me', () => (expired ? unauthorized() : HttpResponse.json(user))),
      http.get('/api/topics/t1', unauthorized),
    )
    const u = userEvent.setup()
    const { router } = renderApp('/')
    await screen.findByRole('heading', { name: 'Your topics' })
    expired = true
    await u.click(screen.getByRole('link', { name: /Biology/ }))
    expect(await screen.findByRole('heading', { name: 'Log in to Quizette' })).toBeInTheDocument()
    await waitFor(() => expect(router.state.location.search).toEqual({ redirect: '/topics/t1' }))
  })
})

describe('sidebar', () => {
  it('lists Upload, Dashboard, Quizzes, History in order and marks the current page', async () => {
    server.use(http.get('/api/chapters', () => HttpResponse.json([])))
    const u = userEvent.setup()
    const { router } = renderApp('/')
    const nav = await screen.findByRole('navigation', { name: 'Main' })
    const links = within(nav).getAllByRole('link')
    expect(links.map((l) => l.textContent)).toEqual(['Upload', 'Dashboard', 'Quizzes', 'History'])
    expect(within(nav).getByRole('link', { name: 'Dashboard' })).toHaveAttribute('aria-current', 'page')

    await u.click(within(nav).getByRole('link', { name: 'Quizzes' }))
    expect(await screen.findByRole('heading', { name: 'Quizzes' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/quizzes')
    expect(within(nav).getByRole('link', { name: 'Quizzes' })).toHaveAttribute('aria-current', 'page')
    expect(within(nav).getByRole('link', { name: 'Dashboard' })).not.toHaveAttribute('aria-current')
  })

  it('is not shown on the login page', async () => {
    server.use(http.get('/api/auth/me', unauthorized))
    renderApp('/login')
    await screen.findByRole('heading', { name: 'Log in to Quizette' })
    expect(screen.queryByRole('navigation', { name: 'Main' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Sign out' })).not.toBeInTheDocument()
  })
})

describe('quizzes', () => {
  it('shows topic cards counting ready chapters, then the topic\'s ready chapters', async () => {
    const requested: string[] = []
    server.use(
      http.get('/api/topics', () =>
        HttpResponse.json([
          { id: 't1', name: 'Biology', chapterCount: 3, createdAt: '2026-11-20T10:00:00Z' },
          { id: 't2', name: 'History', chapterCount: 1, createdAt: '2026-11-20T10:00:00Z' },
        ]),
      ),
      http.get('/api/chapters', ({ request }) => {
        const url = new URL(request.url)
        requested.push(url.search)
        // The server filters; return what it would for each query.
        if (url.searchParams.get('topicId') === 't1') {
          return HttpResponse.json([chapterSummary(), chapterSummary({ id: 'c2', title: 'Genetics', bestScorePct: null })])
        }
        return HttpResponse.json([
          chapterSummary(),
          chapterSummary({ id: 'c2', title: 'Genetics', bestScorePct: null }),
          chapterSummary({ id: 'c3', topicId: 't2', topicName: 'History', title: 'Rome', bestScorePct: null }),
        ])
      }),
    )
    const u = userEvent.setup()
    const { router } = renderApp('/quizzes')
    const main = await screen.findByRole('main')
    const biology = await within(main).findByRole('link', { name: /Biology/ })
    expect(biology).toHaveTextContent('2 chapters')
    expect(within(main).getByRole('link', { name: /History/ })).toHaveTextContent('1 chapter')
    expect(within(main).queryByRole('link', { name: /Start quiz/ })).not.toBeInTheDocument()

    await u.click(biology)
    expect(await screen.findByRole('heading', { name: 'Biology' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/quizzes/t1')
    expect(screen.getByText(/10 questions · 2 attempts · best 80%/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Start quiz: Cells' })).toHaveAttribute('href', '/chapters/c1')
    expect(screen.getByRole('link', { name: 'View questions: Cells' })).toHaveAttribute('href', '/chapters/c1/questions')
    expect(screen.getByRole('link', { name: 'Start quiz: Genetics' })).toBeInTheDocument()
    // Only ready chapters are ever requested.
    expect(requested).toEqual(['?status=ready', '?status=ready&topicId=t1'])
    const nav = screen.getByRole('navigation', { name: 'Main' })
    expect(within(nav).getByRole('link', { name: 'Quizzes' })).toHaveAttribute('aria-current', 'page')
  })

  it('points to upload when no chapters are ready', async () => {
    server.use(http.get('/api/chapters', () => HttpResponse.json([])))
    renderApp('/quizzes')
    expect(await screen.findByText('No quizzes yet')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Upload a PDF' })).toHaveAttribute('href', '/upload')
  })
})

describe('view questions', () => {
  it('walks through the bank and reveals each answer only on request', async () => {
    const questions = attempt().questions.map((q) => ({
      ...q,
      correct: q.type === 'mrq' ? [0, 2] : [q.type === 'mcq' ? 1 : 0],
      explanation: `Why ${q.id}.`,
    }))
    server.use(
      http.get('/api/chapters/c1', () =>
        HttpResponse.json({ id: 'c1', title: 'Cells', status: 'ready', topicId: 't1', topicName: 'Biology', questionCount: 3, latestJob: null, createdAt: '2026-11-20T10:00:00Z' }),
      ),
      http.get('/api/chapters/c1/question-set', () => HttpResponse.json({ id: 's1', chapterId: 'c1', questions })),
    )
    const u = userEvent.setup()
    renderApp('/chapters/c1/questions')

    expect(await screen.findByRole('heading', { name: 'Powerhouse of the cell?' })).toBeInTheDocument()
    expect(screen.getByText('Question 1 of 3')).toBeInTheDocument()
    // No options to pick, and the answer is hidden.
    expect(screen.queryByRole('radio')).not.toBeInTheDocument()
    expect(screen.queryByText('Mitochondria')).not.toBeInTheDocument()

    await u.click(screen.getByRole('button', { name: 'Show answer' }))
    expect(screen.getByText('Mitochondria')).toBeInTheDocument()
    expect(screen.getByText('Why q1.')).toBeInTheDocument()

    // Skip question 2 without revealing it.
    await u.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('Question 2 of 3')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Show answer' })).toBeInTheDocument()
    expect(screen.queryByText('Why q2.')).not.toBeInTheDocument()

    await u.click(screen.getByRole('button', { name: 'Next' }))
    await u.click(screen.getByRole('button', { name: 'Show answer' }))
    expect(screen.getByText('Answers')).toBeInTheDocument()
    expect(screen.getByText('Nucleus')).toBeInTheDocument()
    expect(screen.getByText('Ribosome')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Done' })).toHaveAttribute('href', '/quizzes/t1')

    // Going back keeps question 1 revealed; hiding works too.
    await u.click(screen.getByRole('button', { name: 'Previous' }))
    await u.click(screen.getByRole('button', { name: 'Previous' }))
    expect(screen.getByText('Mitochondria')).toBeInTheDocument()
    await u.click(screen.getByRole('button', { name: 'Hide answer' }))
    expect(screen.queryByText('Mitochondria')).not.toBeInTheDocument()
  })
})

describe('history page', () => {
  it('lists attempts across chapters with review and resume links', async () => {
    server.use(
      http.get('/api/attempts', () =>
        HttpResponse.json([
          historyItem({ id: 'a2', status: 'in_progress', totalScore: 0, scorePct: 0, submittedAt: null, chapterTitle: 'Rome', topicName: 'History' }),
          historyItem(),
        ]),
      ),
    )
    renderApp('/history')
    const rows = (await screen.findAllByRole('row')).slice(1)
    expect(rows).toHaveLength(2)
    const [inProgress, done] = rows
    if (inProgress === undefined || done === undefined) throw new Error('missing rows')
    expect(within(inProgress).getByText('Rome')).toBeInTheDocument()
    expect(within(inProgress).getByText('In progress')).toBeInTheDocument()
    expect(within(inProgress).getByRole('link', { name: 'Resume' })).toHaveAttribute('href', '/attempts/a2')
    expect(within(done).getByText('Biology')).toBeInTheDocument()
    expect(done).toHaveTextContent('2.5 / 5 (50%)')
    expect(within(done).getByRole('link', { name: 'Review' })).toHaveAttribute('href', '/attempts/a1/results')
  })
})

// Regression: opening the mobile menu left keyboard focus on the page behind its
// overlay, so Tab moved through links the user couldn't see.
describe('mobile menu', () => {
  it('moves focus into the menu, keeps Tab inside it, and returns focus on Escape', async () => {
    const u = userEvent.setup()
    renderApp('/')
    const openButton = await screen.findByRole('button', { name: 'Open menu' })
    await u.click(openButton)

    const menu = await screen.findByRole('dialog', { name: 'Menu' })
    expect(menu).toContainElement(document.activeElement as HTMLElement)
    for (let i = 0; i < 12; i++) {
      await u.tab()
      expect(menu).toContainElement(document.activeElement as HTMLElement)
    }

    await u.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Menu' })).not.toBeInTheDocument())
    expect(openButton).toHaveFocus()
  })
})
