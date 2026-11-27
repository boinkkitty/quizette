import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import type { Attempt, Question } from '../api/schemas'
import { formatDate } from '../lib/format'
import { renderApp } from './app'
import { attempt, server } from './server'

const submittedAt = '2026-11-21T10:05:00Z'

/** A graded chapter quiz on Cells (in Biology): 1.5 of 3 points. */
const chapterQuiz: Attempt = attempt({
  totalScore: 1.5,
  maxScore: 3,
  scorePct: 50,
  submittedAt,
  questions: attempt().questions.map((q) => ({
    ...q,
    correct: q.type === 'mrq' ? [0, 2] : [q.type === 'mcq' ? 1 : 0],
    chapterId: 'c1',
    chapterTitle: 'Cells',
  })),
  answers: [
    { questionId: 'q1', selected: [1], points: 1, correct: true },
    { questionId: 'q2', selected: [1], points: 0, correct: false },
    { questionId: 'q3', selected: [0], points: 0.5, correct: false },
  ],
})

/**
 * A graded quiz on all of Biology: Question 1 and 3 from Cells (1.5 of 2, 75%),
 * Question 2 and 4 from Genes (1 of 2, 50%).
 */
const topicQuiz: Attempt = {
  ...chapterQuiz,
  scope: 'topic',
  chapterId: undefined,
  chapterTitle: undefined,
  totalScore: 2.5,
  maxScore: 4,
  scorePct: 62.5,
  questions: (
    [
      ...chapterQuiz.questions,
      {
        id: 'q4',
        position: 3,
        type: 'mcq' as const,
        prompt: 'Carrier of genes?',
        options: ['DNA', 'Fat', 'Salt', 'Sugar'],
        correct: [0],
      },
    ] satisfies Question[]
  ).map((q, i) => ({
    ...q,
    chapterId: i % 2 === 0 ? 'c1' : 'c2',
    chapterTitle: i % 2 === 0 ? 'Cells' : 'Genes',
  })),
  answers: [...chapterQuiz.answers, { questionId: 'q4', selected: [0], points: 1, correct: true }],
}

function serveAttempt(a: Attempt) {
  server.use(http.get('/api/attempts/a1', () => HttpResponse.json(a)))
}

const chapterChips = () => screen.getByRole('group', { name: 'Chapters' })

/** Each shown answer as "<source line> | Question <n>", in order. */
function shownAnswers(): string[] {
  const list = screen.getByRole('list', { name: 'Answers' })
  return within(list)
    .queryAllByRole('listitem')
    .filter((li) => li.parentElement === list)
    .map((li) => {
      const source = li.querySelector('[data-source]')?.textContent ?? ''
      const number = /Question \d+/.exec(li.textContent ?? '')?.[0] ?? ''
      return source === '' ? number : `${source} | ${number}`
    })
}

const bottomBar = () => screen.getByRole('navigation', { name: 'Result actions' })

describe('results page', () => {
  it('shows a chapter quiz with Retake, Go to the chapter, and History', async () => {
    serveAttempt(chapterQuiz)
    renderApp('/history/a1')

    expect(await screen.findByRole('heading', { name: 'Cells' })).toBeInTheDocument()
    expect(
      screen.getByText(`Chapter quiz · Biology · ${formatDate(submittedAt)}`),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '← History' })).toHaveAttribute('href', '/history')
    expect(screen.getByTestId('score')).toHaveTextContent('1.5 / 3')

    const bar = within(bottomBar())
    expect(bar.getByRole('link', { name: 'Retake' })).toHaveAttribute(
      'href',
      '/quizzes?chapterId=c1',
    )
    expect(bar.getByRole('link', { name: 'Go to Cells' })).toHaveAttribute('href', '/chapters/c1')
    expect(bar.getByRole('link', { name: 'History' })).toHaveAttribute('href', '/history')
  })

  it('shows a topic quiz with Retake and Go to the topic', async () => {
    serveAttempt(topicQuiz)
    renderApp('/history/a1')

    expect(await screen.findByRole('heading', { name: 'Biology' })).toBeInTheDocument()
    expect(
      screen.getByText(`Whole-topic quiz · 2 chapters · ${formatDate(submittedAt)}`),
    ).toBeInTheDocument()

    const bar = within(bottomBar())
    expect(bar.getByRole('link', { name: 'Retake' })).toHaveAttribute('href', '/quizzes?topicId=t1')
    expect(bar.getByRole('link', { name: 'Go to Biology' })).toHaveAttribute('href', '/topics/t1')
  })
  it('filters a topic quiz by chapter, keeping the original question numbers', async () => {
    serveAttempt(topicQuiz)
    const u = userEvent.setup()
    renderApp('/history/a1')
    await screen.findByRole('heading', { name: 'Biology' })

    const chips = within(chapterChips())
    expect(chips.getAllByRole('button').map((b) => b.textContent)).toEqual([
      'All chapters',
      'Cells75%',
      'Genes50%',
    ])
    expect(shownAnswers()).toEqual([
      'From Cells | Question 1',
      'From Genes | Question 2',
      'From Cells | Question 3',
      'From Genes | Question 4',
    ])
    expect(screen.getAllByRole('link', { name: 'Genes' })[0]).toHaveAttribute(
      'href',
      '/chapters/c2',
    )

    await u.click(chips.getByRole('button', { name: /Genes/ }))
    expect(shownAnswers()).toEqual(['Question 2', 'Question 4'])
    expect(screen.getByRole('link', { name: 'Open chapter →' })).toHaveAttribute(
      'href',
      '/chapters/c2',
    )

    await u.click(chips.getByRole('button', { name: 'All chapters' }))
    expect(shownAnswers()).toHaveLength(4)
    expect(screen.queryByRole('link', { name: 'Open chapter →' })).not.toBeInTheDocument()
  })

  it('has no chapter chips or source lines for a chapter quiz', async () => {
    serveAttempt(chapterQuiz)
    renderApp('/history/a1')
    await screen.findByRole('heading', { name: 'Cells' })
    expect(screen.queryByRole('group', { name: 'Chapters' })).not.toBeInTheDocument()
    expect(shownAnswers()).toEqual(['Question 1', 'Question 2', 'Question 3'])
  })
  it('only shows mistakes, together with the chapter filter', async () => {
    serveAttempt(topicQuiz)
    const u = userEvent.setup()
    renderApp('/history/a1')
    const mistakes = await screen.findByRole('checkbox', { name: 'Only show mistakes' })

    // Question 3 earned partial credit, so it counts as a mistake.
    await u.click(mistakes)
    expect(shownAnswers()).toEqual(['From Genes | Question 2', 'From Cells | Question 3'])
    await u.click(within(chapterChips()).getByRole('button', { name: /Genes/ }))
    expect(shownAnswers()).toEqual(['Question 2'])

    await u.click(mistakes)
    expect(shownAnswers()).toEqual(['Question 2', 'Question 4'])
  })

  it('says when there are no mistakes to show', async () => {
    serveAttempt({
      ...chapterQuiz,
      answers: chapterQuiz.answers.map((a) => ({ ...a, points: 1, correct: true })),
    })
    const u = userEvent.setup()
    renderApp('/history/a1')
    await u.click(await screen.findByRole('checkbox', { name: 'Only show mistakes' }))
    expect(shownAnswers()).toEqual([])
    expect(screen.getByText('No mistakes to show.')).toBeInTheDocument()
  })
  it('keeps a deleted chapter of a topic quiz, without links to it', async () => {
    serveAttempt({
      ...topicQuiz,
      questions: topicQuiz.questions.map((q) =>
        q.chapterId === 'c2' ? { ...q, chapterDeleted: true } : q,
      ),
    })
    const u = userEvent.setup()
    renderApp('/history/a1')
    await screen.findByRole('heading', { name: 'Biology' })

    const genes = within(chapterChips()).getByRole('button', { name: /Genes/ })
    expect(genes).toHaveTextContent('Deleted chapter: Genes50%')
    expect(shownAnswers()).toEqual([
      'From Cells | Question 1',
      'From a deleted chapter: Genes | Question 2',
      'From Cells | Question 3',
      'From a deleted chapter: Genes | Question 4',
    ])
    expect(screen.queryByRole('link', { name: 'Genes' })).not.toBeInTheDocument()

    await u.click(genes)
    expect(shownAnswers()).toEqual(['Question 2', 'Question 4'])
    expect(
      screen.getByText('Genes was deleted. Its questions stay here for this result.'),
    ).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Open chapter →' })).not.toBeInTheDocument()

    // The topic itself still exists, so it can be retaken.
    expect(within(bottomBar()).getByRole('link', { name: 'Retake' })).toBeInTheDocument()
    expect(within(bottomBar()).getByRole('link', { name: 'Go to Biology' })).toBeInTheDocument()
  })
  it.each([
    ['chapter', chapterQuiz],
    ['topic', topicQuiz],
  ] as const)('replaces Retake and Go to once the quiz’s %s is deleted', async (what, quiz) => {
    serveAttempt({
      ...quiz,
      scopeDeleted: true,
      questions: quiz.questions.map((q) => ({ ...q, chapterDeleted: true })),
    })
    renderApp('/history/a1')
    expect(await screen.findByTestId('score')).toBeInTheDocument()

    const bar = within(bottomBar())
    expect(bar.getByText(`This ${what} was deleted, so it can't be retaken.`)).toBeInTheDocument()
    expect(bar.queryByRole('link', { name: 'Retake' })).not.toBeInTheDocument()
    expect(bar.queryByRole('link', { name: /^Go to/ })).not.toBeInTheDocument()
    expect(bar.getByRole('link', { name: 'History' })).toHaveAttribute('href', '/history')
  })
})
