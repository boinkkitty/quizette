import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { ChapterSummary, Topic } from '../api/schemas'
import { renderApp } from './app'
import { attempt, chapterSummary, server, topic, topicQuiz } from './server'

const biology = topic({
  id: 't1',
  name: 'Biology',
  chapterCount: 3,
  attemptCount: 2,
  lastScorePct: 60,
  bestScorePct: 90,
})

const cells = chapterSummary({ id: 'c1', title: 'Cells', questionCount: 10 })
const genetics = chapterSummary({
  id: 'c2',
  title: 'Genetics',
  questionCount: 20,
  attemptCount: 0,
  bestScorePct: null,
  createdAt: '2026-11-21T10:00:00Z',
})

/** Serves the topics and every chapter (all statuses) the picker loads, and records draws. */
function serveLibrary(topics: Topic[], chapters: ChapterSummary[]) {
  const draws: unknown[] = []
  server.use(
    http.get('/api/topics', () => HttpResponse.json(topics)),
    http.get('/api/chapters', () => HttpResponse.json(chapters)),
    http.post('/api/quizzes/draw', async ({ request }) => {
      draws.push(await request.json())
      return HttpResponse.json(topicQuiz())
    }),
  )
  return draws
}

describe('quiz picker', () => {
  it('starts a whole-topic quiz: topic, then Whole topic, then how many', async () => {
    const draws = serveLibrary([biology], [cells, genetics])
    const u = userEvent.setup()
    const { router } = renderApp('/quizzes')

    const topicCard = await screen.findByRole('button', { name: /Biology/ })
    expect(topicCard).toHaveTextContent('2 chapters · 30 questions')
    expect(topicCard).toHaveTextContent('2 quizzes · last 60% · best 90%')
    await u.click(topicCard)

    await u.click(screen.getByRole('button', { name: /Whole topic/ }))
    expect(screen.getByRole('slider', { name: 'Number of questions' })).toHaveValue('10')
    await u.click(screen.getByRole('button', { name: 'Start 10-question quiz' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/quizzes/run'))
    expect(draws).toEqual([{ topicId: 't1', count: 10 }])
    expect(await screen.findByRole('heading', { name: 'All of Biology' })).toBeInTheDocument()
  })

  it('marks the current step in the step indicator', async () => {
    serveLibrary([biology], [cells, genetics])
    const u = userEvent.setup()
    renderApp('/quizzes')
    const current = () =>
      within(screen.getByRole('list', { name: 'Steps' })).getByText(
        (_, el) => el?.getAttribute('aria-current') === 'step',
      )

    await u.click(await screen.findByRole('button', { name: /Biology/ }))
    expect(current()).toHaveTextContent('What to quiz')
    await u.click(screen.getByRole('button', { name: /Whole topic/ }))
    expect(current()).toHaveTextContent('How many')
    await u.click(screen.getByRole('button', { name: 'Back' }))
    await u.click(screen.getByRole('button', { name: 'Back' }))
    expect(current()).toHaveTextContent('Topic')
  })

  it('opens step 3 with the chapter from ?chapterId=, and Back goes to its topic', async () => {
    const draws = serveLibrary([biology], [cells, genetics])
    const u = userEvent.setup()
    renderApp('/quizzes?chapterId=c2')

    expect(await screen.findByText('Genetics')).toBeInTheDocument()
    expect(screen.getByRole('slider', { name: 'Number of questions' })).toBeInTheDocument()

    await u.click(screen.getByRole('button', { name: 'Back' }))
    expect(screen.getByRole('button', { name: /Whole topic: Biology/ })).toBeInTheDocument()

    await u.click(screen.getByRole('button', { name: /Genetics/ }))
    await u.click(screen.getByRole('button', { name: 'Start 10-question quiz' }))
    await waitFor(() => expect(draws).toEqual([{ chapterId: 'c2', count: 10 }]))
  })

  it('opens step 3 with the whole topic from ?topicId=, and Back goes to its chapters', async () => {
    serveLibrary([biology], [cells, genetics])
    const u = userEvent.setup()
    renderApp('/quizzes?topicId=t1')

    expect(await screen.findByText('All of Biology')).toBeInTheDocument()
    expect(screen.getByRole('slider', { name: 'Number of questions' })).toBeInTheDocument()
    await u.click(screen.getByRole('button', { name: 'Back' }))
    expect(screen.getByRole('button', { name: /Genetics/ })).toBeInTheDocument()
  })

  const pending = chapterSummary({
    id: 'c3',
    title: 'Ecology',
    status: 'pending',
    questionCount: 0,
  })
  const empty = topic({ id: 't2', name: 'History', chapterCount: 1 })
  const rome = chapterSummary({ id: 'c4', topicId: 't2', title: 'Rome', status: 'failed' })

  it('disables a topic with no ready chapter, and says how many chapters are not ready', async () => {
    serveLibrary([biology, empty], [cells, genetics, pending, rome])
    const u = userEvent.setup()
    renderApp('/quizzes')

    const history = await screen.findByRole('button', { name: /History/ })
    expect(history).toBeDisabled()
    expect(history).toHaveTextContent('No chapters ready yet')

    await u.click(screen.getByRole('button', { name: /Biology/ }))
    expect(screen.getByText('1 more chapter is not ready yet.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Ecology/ })).not.toBeInTheDocument()
  })

  it('leads to Upload instead of the steps when no chapter is ready anywhere', async () => {
    serveLibrary([biology, empty], [pending, rome])
    renderApp('/quizzes?topicId=t1')

    expect(await screen.findByText('Nothing to quiz yet')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Upload a PDF' })).toHaveAttribute('href', '/upload')
    expect(screen.queryByRole('button', { name: /Biology/ })).not.toBeInTheDocument()
  })

  // A chapter's bank is a multiple of 5, but a topic's pool need not be.
  it.each([
    ['a 5-question chapter', [5], 5, '5'],
    ['a 10-question chapter', [10], 10, '10'],
    ['a 23-question pool, rounded down', [8, 15], 20, '10'],
    ['a pool over 50, capped', [20, 20, 20], 50, '10'],
  ])('bounds the slider for %s', async (_case, sizes, max, initial) => {
    const chapters = sizes.map((n, i) =>
      chapterSummary({ id: `c${String(i)}`, title: `Chapter ${String(i)}`, questionCount: n }),
    )
    serveLibrary([biology], chapters)
    renderApp(sizes.length === 1 ? '/quizzes?chapterId=c0' : '/quizzes?topicId=t1')

    const slider = await screen.findByRole('slider', { name: 'Number of questions' })
    expect(slider).toHaveAttribute('min', '5')
    expect(slider).toHaveAttribute('max', String(max))
    expect(slider).toHaveAttribute('step', '5')
    expect(slider).toHaveValue(initial)
    expect(screen.getByRole('button', { name: `Start ${initial}-question quiz` })).toBeEnabled()
  })

  /** The preview's rows as "<chapter><count>", in display order. */
  const previewRows = () =>
    within(screen.getByRole('list', { name: 'Where they come from' }))
      .getAllByRole('listitem')
      .map((li) => li.textContent)

  const setCount = (n: number) => {
    fireEvent.change(screen.getByRole('slider', { name: 'Number of questions' }), {
      target: { value: String(n) },
    })
  }

  it('previews the server split for uneven chapter sizes', async () => {
    const ecology = chapterSummary({
      id: 'c3',
      title: 'Ecology',
      questionCount: 15,
      createdAt: '2026-11-22T10:00:00Z',
    })
    serveLibrary(
      [biology],
      [chapterSummary({ questionCount: 5 }), { ...genetics, questionCount: 10 }, ecology],
    )
    renderApp('/quizzes?topicId=t1')

    // Sizes 5, 10, 15 of 30 (TestSplitAcrossChapters "uneven sizes").
    await screen.findByRole('slider', { name: 'Number of questions' })
    expect(previewRows()).toEqual(['Cells2', 'Genetics3', 'Ecology5'])
    setCount(5) // quotas 0.83, 1.67, 2.5: the two largest remainders get the leftovers
    expect(previewRows()).toEqual(['Cells1', 'Genetics2', 'Ecology2'])
  })

  it('splits in creation order, so a remainder tie goes to the older chapter', async () => {
    const older = chapterSummary({ id: 'c9', title: 'Older', questionCount: 7 })
    const newer = chapterSummary({
      id: 'c1',
      title: 'Newer',
      questionCount: 3,
      createdAt: '2026-11-21T10:00:00Z',
    })
    serveLibrary([biology], [newer, older])
    renderApp('/quizzes?topicId=t1')

    await screen.findByRole('slider', { name: 'Number of questions' })
    setCount(5) // quotas 3.5 and 1.5 (TestSplitAcrossChapters "tie")
    expect(previewRows()).toEqual(['Older4', 'Newer1'])
  })

  it('has no split preview for a chapter quiz', async () => {
    serveLibrary([biology], [cells, genetics])
    renderApp('/quizzes?chapterId=c1')
    await screen.findByRole('slider', { name: 'Number of questions' })
    expect(screen.queryByRole('list', { name: 'Where they come from' })).not.toBeInTheDocument()
  })

  it.each([
    ['an unknown chapter', '?chapterId=nope'],
    ['a chapter that is not ready', '?chapterId=c3'],
    ['an unknown topic', '?topicId=nope'],
    ['a topic with no ready chapter', '?topicId=t2'],
  ])('ignores %s and starts at step 1', async (_case, search) => {
    serveLibrary([biology, empty], [cells, genetics, pending, rome])
    renderApp(`/quizzes${search}`)

    expect(await screen.findByRole('button', { name: /Biology/ })).toBeInTheDocument()
    expect(screen.queryByRole('slider')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Back' })).not.toBeInTheDocument()
  })
})

describe('topic quiz', () => {
  const graded = attempt({
    scope: 'topic',
    chapterId: undefined,
    chapterTitle: undefined,
    topicId: 't1',
    topicName: 'Biology',
    totalScore: 1,
    scorePct: 20,
  })

  /** Starts a 10-question whole-topic quiz from the picker; returns the submit bodies. */
  async function startTopicQuiz(u: ReturnType<typeof userEvent.setup>) {
    serveLibrary([biology], [cells, genetics])
    const submits: unknown[] = []
    server.use(
      http.post('/api/attempts', async ({ request }) => {
        submits.push(await request.json())
        return HttpResponse.json(graded, { status: 201 })
      }),
    )
    const app = renderApp('/quizzes?topicId=t1')
    await u.click(await screen.findByRole('button', { name: 'Start 10-question quiz' }))
    expect(await screen.findByRole('heading', { name: 'All of Biology' })).toBeInTheDocument()
    return { ...app, submits }
  }

  it('submits with the topic scope and shows the results', async () => {
    const u = userEvent.setup()
    const { router, submits } = await startTopicQuiz(u)

    await u.click(screen.getByRole('radio', { name: 'Mitochondria' }))
    await u.click(screen.getByRole('button', { name: 'Next' }))
    await u.click(screen.getByRole('button', { name: 'Next' }))
    await u.click(screen.getByRole('button', { name: 'Submit quiz' }))
    await u.click(await screen.findByRole('button', { name: 'Submit anyway' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/attempts/a1/results'))
    expect(submits).toEqual([
      {
        topicId: 't1',
        questionIds: ['q1', 'q2', 'q3'],
        answers: [{ questionId: 'q1', selected: [1] }],
      },
    ])
    expect(await screen.findByRole('link', { name: '← All of Biology' })).toHaveAttribute(
      'href',
      '/topics/t1',
    )
  })

  it('quits back to the picker with the topic preselected', async () => {
    const u = userEvent.setup()
    const { router } = await startTopicQuiz(u)

    await u.click(screen.getByRole('button', { name: 'Quit' }))
    await u.click(screen.getByRole('button', { name: 'Quit quiz' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/quizzes'))
    expect(router.state.location.search).toEqual({ topicId: 't1' })
    expect(await screen.findByText('All of Biology')).toBeInTheDocument()
  })
})
