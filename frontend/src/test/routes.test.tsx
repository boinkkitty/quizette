import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { Attempt } from '../api/schemas'
import { startQuiz } from '../lib/quizSlot'
import { renderApp } from './app'
import {
  attempt,
  chapterSummary,
  drawnQuiz,
  historyItem,
  server,
  topicQuiz,
  unauthorized,
  user,
} from './server'

describe('routing', () => {
  it('redirects logged-out users to login, preserving the destination', async () => {
    server.use(http.get('/api/auth/me', unauthorized))
    const { router } = renderApp('/upload')
    expect(await screen.findByRole('heading', { name: 'Log in to Quizette' })).toBeInTheDocument()
    expect(router.state.location.search).toEqual({ redirect: '/upload' })
    expect(await screen.findByRole('link', { name: 'Continue with GitHub' })).toHaveAttribute(
      'href',
      '/api/auth/github/start',
    )
  })

  it('shows Google before GitHub whatever order the API lists them', async () => {
    server.use(
      http.get('/api/auth/me', unauthorized),
      http.get('/api/auth/providers', () => HttpResponse.json({ providers: ['github', 'google'] })),
    )
    renderApp('/login')
    const github = await screen.findByRole('link', { name: 'Continue with GitHub' })
    const links = screen.getAllByRole('link', { name: /^Continue with/ })
    expect(links.map((l) => l.textContent)).toEqual([
      'Continue with Google',
      'Continue with GitHub',
    ])
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
      http.post('/api/auth/login', () =>
        HttpResponse.json(
          { error: 'unauthorized', message: 'invalid email or password' },
          { status: 401 },
        ),
      ),
    )
    const u = userEvent.setup()
    renderApp('/login')
    await u.type(await screen.findByLabelText('Email'), 'ada@example.com')
    await u.type(screen.getByLabelText('Password'), 'wrong')
    await u.click(screen.getByRole('button', { name: 'Log in' }))
    expect(await screen.findByText('invalid email or password')).toBeInTheDocument()
  })
})

describe('quiz runner', () => {
  const submitted: Attempt = attempt({
    totalScore: 2.5,
    scorePct: 50,
    questions: attempt().questions.map((q) => ({
      ...q,
      correct: q.type === 'mrq' ? [0, 2] : [q.type === 'mcq' ? 1 : 0],
      explanation: 'Because.',
    })),
    answers: [
      { questionId: 'q1', selected: [1], points: 1, correct: true },
      { questionId: 'q2', selected: [1], points: 0, correct: false },
      { questionId: 'q3', selected: [0], points: 0.5, correct: false },
    ],
  })

  /** Serves the quiz picker's chapters and the draw; returns the draw requests' bodies. */
  function serveChapter() {
    const draws: unknown[] = []
    server.use(
      http.get('/api/chapters', () => HttpResponse.json([chapterSummary()])),
      http.post('/api/quizzes/draw', async ({ request }) => {
        draws.push(await request.json())
        return HttpResponse.json(drawnQuiz())
      }),
      http.get('/api/attempts/a1', () => HttpResponse.json(submitted)),
    )
    return draws
  }

  it('starts from Quiz this chapter, survives a reload, and submits to the results', async () => {
    const draws = serveChapter()
    const submits: unknown[] = []
    server.use(
      http.post('/api/attempts', async ({ request }) => {
        submits.push(await request.json())
        return HttpResponse.json(submitted, { status: 201 })
      }),
    )
    const u = userEvent.setup()
    const first = renderApp('/quizzes?chapterId=c1')
    fireEvent.change(await screen.findByRole('slider', { name: 'Number of questions' }), {
      target: { value: '5' },
    })
    await u.click(screen.getByRole('button', { name: 'Start 5-question quiz' }))
    await waitFor(() => expect(first.router.state.location.pathname).toBe('/quizzes/run'))
    expect(draws).toEqual([{ chapterId: 'c1', count: 5 }])
    expect(await screen.findByRole('heading', { name: 'Cells' })).toBeInTheDocument()

    await u.click(await screen.findByRole('radio', { name: 'Mitochondria' }))
    await u.click(screen.getByRole('button', { name: 'Next' }))
    await u.click(screen.getByRole('radio', { name: 'False' }))
    first.unmount() // the page reloads

    const { router } = renderApp('/quizzes/run')
    expect(await screen.findByText('Question 2 of 3')).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: 'False' })).toBeChecked()
    await u.click(screen.getByRole('button', { name: 'Previous' }))
    expect(screen.getByRole('radio', { name: 'Mitochondria' })).toBeChecked()
    await u.click(screen.getByRole('button', { name: 'Next' }))
    await u.click(screen.getByRole('button', { name: 'Next' }))
    await u.click(screen.getByRole('checkbox', { name: 'Nucleus' }))
    await u.click(screen.getByRole('button', { name: 'Submit quiz' }))

    expect(await screen.findByTestId('score')).toHaveTextContent('2.5 / 5')
    expect(router.state.location.pathname).toBe('/attempts/a1/results')
    expect(screen.getByText('0.5 / 1 point')).toBeInTheDocument()
    expect(submits).toEqual([
      {
        chapterId: 'c1',
        questionIds: ['q1', 'q2', 'q3'],
        answers: [
          { questionId: 'q1', selected: [1] },
          { questionId: 'q2', selected: [1] },
          { questionId: 'q3', selected: [0] },
        ],
      },
    ])
    expect(window.sessionStorage.length).toBe(0) // the slot is cleared once submitted
  })

  it('replaces the runner with the results in the history', async () => {
    serveChapter()
    server.use(http.post('/api/attempts', () => HttpResponse.json(submitted, { status: 201 })))
    const u = userEvent.setup()
    const { router } = renderApp('/quizzes?chapterId=c1')
    await u.click(await screen.findByRole('button', { name: 'Start 10-question quiz' }))
    await u.click(await screen.findByRole('button', { name: 'Next' }))
    await u.click(screen.getByRole('button', { name: 'Next' }))
    await u.click(screen.getByRole('button', { name: 'Submit quiz' }))
    await u.click(await screen.findByRole('button', { name: 'Submit anyway' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/attempts/a1/results'))

    router.history.back()
    await waitFor(() => expect(router.state.location.pathname).toBe('/quizzes'))
  })

  it('sends an empty tab to the quizzes page', async () => {
    const { router } = renderApp('/quizzes/run')
    await waitFor(() => expect(router.state.location.pathname).toBe('/quizzes'))
  })

  it('quits after confirming without recording anything', async () => {
    serveChapter()
    let recorded = false
    server.use(
      http.post('/api/attempts', () => {
        recorded = true
        return HttpResponse.json({}, { status: 500 })
      }),
    )
    const u = userEvent.setup()
    const { router } = renderApp('/quizzes?chapterId=c1')
    await u.click(await screen.findByRole('button', { name: 'Start 10-question quiz' }))
    await u.click(await screen.findByRole('radio', { name: 'Mitochondria' }))

    await u.click(screen.getByRole('button', { name: 'Quit' }))
    expect(screen.getByText("Your answers won't be saved.")).toBeInTheDocument()
    await u.click(screen.getByRole('button', { name: 'Keep answering' }))
    expect(screen.getByRole('radio', { name: 'Mitochondria' })).toBeChecked()

    await u.click(screen.getByRole('button', { name: 'Quit' }))
    await u.click(screen.getByRole('button', { name: 'Quit quiz' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/quizzes'))
    expect(router.state.location.search).toEqual({ chapterId: 'c1' })
    expect(window.sessionStorage.length).toBe(0)
    expect(recorded).toBe(false)
  })

  it('keeps the answers when submitting fails, and can retry', async () => {
    serveChapter()
    let calls = 0
    server.use(
      http.post('/api/attempts', () => {
        calls++
        return calls === 1 ? HttpResponse.error() : HttpResponse.json(submitted, { status: 201 })
      }),
    )
    const u = userEvent.setup()
    const { router } = renderApp('/quizzes?chapterId=c1')
    await u.click(await screen.findByRole('button', { name: 'Start 10-question quiz' }))
    for (const name of ['Mitochondria', 'False']) {
      await u.click(await screen.findByRole('radio', { name }))
      await u.click(screen.getByRole('button', { name: 'Next' }))
    }
    await u.click(screen.getByRole('checkbox', { name: 'Nucleus' }))
    await u.click(screen.getByRole('button', { name: 'Submit quiz' }))

    expect(await screen.findByText('Could not submit')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/quizzes/run')
    expect(screen.getByRole('checkbox', { name: 'Nucleus' })).toBeChecked()
    expect(window.sessionStorage.length).toBe(1)

    await u.click(screen.getByRole('button', { name: 'Submit quiz' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/attempts/a1/results'))
    expect(calls).toBe(2)
  })

  it('can only discard a quiz whose questions were deleted', async () => {
    serveChapter()
    server.use(
      http.post('/api/attempts', () =>
        HttpResponse.json(
          { error: 'content_deleted', message: 'some of these questions were deleted' },
          { status: 409 },
        ),
      ),
    )
    const u = userEvent.setup()
    const { router } = renderApp('/quizzes?chapterId=c1')
    await u.click(await screen.findByRole('button', { name: 'Start 10-question quiz' }))
    for (const name of ['Mitochondria', 'False']) {
      await u.click(await screen.findByRole('radio', { name }))
      await u.click(screen.getByRole('button', { name: 'Next' }))
    }
    await u.click(screen.getByRole('checkbox', { name: 'Nucleus' }))
    await u.click(screen.getByRole('button', { name: 'Submit quiz' }))

    expect(
      await screen.findByText("Some of these questions were deleted, so this quiz can't be saved."),
    ).toBeInTheDocument()
    expect(screen.queryByText('Could not submit')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Submit quiz' })).not.toBeInTheDocument()

    await u.click(screen.getByRole('button', { name: 'Discard' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/quizzes'))
    expect(window.sessionStorage.length).toBe(0)
  })

  // Regression: the page read chapterTitle, which a whole-topic quiz doesn't have.
  it('names a whole-topic quiz whose questions were deleted', async () => {
    server.use(
      http.post('/api/attempts', () =>
        HttpResponse.json(
          { error: 'content_deleted', message: 'some of these questions were deleted' },
          { status: 409 },
        ),
      ),
    )
    startQuiz(topicQuiz())
    const u = userEvent.setup()
    renderApp('/quizzes/run')
    for (let i = 0; i < 2; i++) await u.click(await screen.findByRole('button', { name: 'Next' }))
    await u.click(screen.getByRole('button', { name: 'Submit quiz' }))
    await u.click(await screen.findByRole('button', { name: 'Submit anyway' }))

    await screen.findByText("Some of these questions were deleted, so this quiz can't be saved.")
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('All of Biology')
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
  it('lists Upload, Dashboard, Topics, Quizzes, History in order and marks the current page', async () => {
    server.use(http.get('/api/chapters', () => HttpResponse.json([])))
    const u = userEvent.setup()
    const { router } = renderApp('/')
    const nav = await screen.findByRole('navigation', { name: 'Main' })
    const links = within(nav).getAllByRole('link')
    expect(links.map((l) => l.textContent)).toEqual([
      'Upload',
      'Dashboard',
      'Topics',
      'Quizzes',
      'History',
    ])
    expect(within(nav).getByRole('link', { name: 'Dashboard' })).toHaveAttribute(
      'aria-current',
      'page',
    )

    await u.click(within(nav).getByRole('link', { name: 'Quizzes' }))
    expect(await screen.findByRole('heading', { name: 'Quizzes' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/quizzes')
    expect(within(nav).getByRole('link', { name: 'Quizzes' })).toHaveAttribute(
      'aria-current',
      'page',
    )
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

describe('view questions', () => {
  it('walks through the bank and reveals each answer only on request', async () => {
    const questions = attempt().questions.map((q) => ({
      ...q,
      correct: q.type === 'mrq' ? [0, 2] : [q.type === 'mcq' ? 1 : 0],
      explanation: `Why ${q.id}.`,
    }))
    server.use(
      http.get('/api/chapters/c1', () =>
        HttpResponse.json({
          id: 'c1',
          title: 'Cells',
          status: 'ready',
          topicId: 't1',
          topicName: 'Biology',
          questionCount: 3,
          latestJob: null,
          createdAt: '2026-11-20T10:00:00Z',
        }),
      ),
      http.get('/api/chapters/c1/question-set', () =>
        HttpResponse.json({ id: 's1', chapterId: 'c1', questions }),
      ),
    )
    const u = userEvent.setup()
    renderApp('/chapters/c1/questions')

    expect(
      await screen.findByRole('heading', { name: 'Powerhouse of the cell?' }),
    ).toBeInTheDocument()
    expect(screen.getByText('Question 1 of 3')).toBeInTheDocument()
    // The options are listed but not selectable, and nothing is marked yet.
    const options = () =>
      within(screen.getByRole('list', { name: 'Options' })).getAllByRole('listitem')
    expect(options().map((o) => o.textContent)).toEqual([
      'ANucleus',
      'BMitochondria',
      'CRibosome',
      'DGolgi',
    ])
    expect(screen.queryByRole('radio')).not.toBeInTheDocument()
    expect(screen.queryByText('Correct')).not.toBeInTheDocument()
    expect(screen.queryByText('Why q1.')).not.toBeInTheDocument()

    await u.click(screen.getByRole('button', { name: 'Show answer' }))
    expect(within(options()[1] as HTMLElement).getByText('Correct')).toBeInTheDocument() // Mitochondria
    expect(screen.getAllByText('Correct')).toHaveLength(1)
    expect(screen.getByText('Why q1.')).toBeInTheDocument()

    // Skip question 2 without revealing it.
    await u.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('Question 2 of 3')).toBeInTheDocument()
    expect(options().map((o) => o.textContent)).toEqual(['ATrue', 'BFalse'])
    expect(screen.queryByText('Correct')).not.toBeInTheDocument()

    // MRQ: every correct option is marked.
    await u.click(screen.getByRole('button', { name: 'Next' }))
    await u.click(screen.getByRole('button', { name: 'Show answer' }))
    const marked = options()
      .filter((o) => within(o).queryByText('Correct') !== null)
      .map((o) => o.textContent)
    expect(marked).toEqual(['NucleusCorrect', 'RibosomeCorrect'])
    expect(screen.getByRole('link', { name: 'Done' })).toHaveAttribute('href', '/chapters/c1')

    // Going back keeps question 1 revealed; hiding works too.
    await u.click(screen.getByRole('button', { name: 'Previous' }))
    await u.click(screen.getByRole('button', { name: 'Previous' }))
    expect(screen.getAllByText('Correct')).toHaveLength(1)
    await u.click(screen.getByRole('button', { name: 'Hide answer' }))
    expect(screen.queryByText('Correct')).not.toBeInTheDocument()
    expect(options()).toHaveLength(4) // options stay visible
  })
})

describe('history page', () => {
  it('lists attempts across chapters with review links', async () => {
    server.use(
      http.get('/api/attempts', () =>
        HttpResponse.json([
          historyItem({ id: 'a2', chapterTitle: 'Rome', topicName: 'History' }),
          historyItem(),
        ]),
      ),
    )
    renderApp('/history')
    const rows = (await screen.findAllByRole('row')).slice(1)
    expect(rows).toHaveLength(2)
    const [rome, cells] = rows
    if (rome === undefined || cells === undefined) throw new Error('missing rows')
    expect(within(rome).getByText('Rome')).toBeInTheDocument()
    expect(within(cells).getByText('Biology')).toBeInTheDocument()
    expect(cells).toHaveTextContent('2.5 / 5 (50%)')
    expect(within(cells).getByRole('link', { name: 'Review' })).toHaveAttribute(
      'href',
      '/attempts/a1/results',
    )
    expect(screen.queryByText('In progress')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Resume' })).not.toBeInTheDocument()
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
    await waitFor(() =>
      expect(screen.queryByRole('dialog', { name: 'Menu' })).not.toBeInTheDocument(),
    )
    expect(openButton).toHaveFocus()
  })
})
