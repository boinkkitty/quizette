import { describe, expect, it } from 'vitest'

import type { DrawnQuiz } from '../api/schemas'
import { clearQuiz, loadQuiz, saveQuizProgress, startQuiz } from './quizSlot'

const quiz = (chapterId: string, ids: string[]): DrawnQuiz => ({
  chapterId,
  chapterTitle: `Chapter ${chapterId}`,
  questions: ids.map((id, position) => ({
    id,
    position,
    type: 'true_false',
    prompt: `Question ${id}?`,
    options: ['True', 'False'],
  })),
})

describe('quiz slot', () => {
  it('holds the drawn quiz with answers and position across a reload', () => {
    startQuiz(quiz('c1', ['q1', 'q2', 'q3']))
    saveQuizProgress({
      index: 2,
      responses: new Map([
        ['q1', [0]],
        ['q3', [1]],
      ]),
    })

    const got = loadQuiz()
    expect(got?.quiz).toEqual(quiz('c1', ['q1', 'q2', 'q3']))
    expect(got?.index).toBe(2)
    expect(Object.fromEntries(got?.responses ?? [])).toEqual({ q1: [0], q3: [1] })
  })

  it('starting a second quiz leaves no answers or position from the first', () => {
    startQuiz(quiz('c1', ['q1', 'q2']))
    saveQuizProgress({ index: 1, responses: new Map([['q1', [0]]]) })

    // The second draw shares a question with the first.
    startQuiz(quiz('c2', ['q1', 'q9']))

    const got = loadQuiz()
    expect(got?.quiz.chapterId).toBe('c2')
    expect(got?.index).toBe(0)
    expect(got?.responses.size).toBe(0)
  })

  it('is empty after clearing, before any quiz, and when the slot is corrupt', () => {
    expect(loadQuiz()).toBeNull()
    startQuiz(quiz('c1', ['q1']))
    clearQuiz()
    expect(loadQuiz()).toBeNull()

    window.sessionStorage.setItem('quizette:quiz', '{"quiz":')
    expect(loadQuiz()).toBeNull()
  })

  it('drops answers to questions not in the quiz and clamps the position', () => {
    startQuiz(quiz('c1', ['q1', 'q2']))
    saveQuizProgress({
      index: 9,
      responses: new Map([
        ['q1', [1]],
        ['gone', [0]],
      ]),
    })
    const got = loadQuiz()
    expect(got?.index).toBe(1)
    expect([...(got?.responses.keys() ?? [])]).toEqual(['q1'])
  })
})
