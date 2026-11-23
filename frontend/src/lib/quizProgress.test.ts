import { describe, expect, it } from 'vitest'

import { clearQuizProgress, loadQuizProgress, saveQuizProgress } from './quizProgress'

describe('quiz progress storage', () => {
  it('round-trips answers and position, and clears them', () => {
    saveQuizProgress('a1', { index: 1, responses: new Map([['q1', [2]], ['q3', [0, 2]]]) })
    const got = loadQuizProgress('a1', ['q1', 'q2', 'q3'])
    expect(got.index).toBe(1)
    expect(Object.fromEntries(got.responses)).toEqual({ q1: [2], q3: [0, 2] })

    clearQuizProgress('a1')
    expect(loadQuizProgress('a1', ['q1']).responses.size).toBe(0)
  })

  it('drops answers to unknown questions and clamps the position', () => {
    saveQuizProgress('a1', { index: 9, responses: new Map([['q1', [1]], ['gone', [0]]]) })
    const got = loadQuizProgress('a1', ['q1', 'q2'])
    expect(got.index).toBe(1)
    expect([...got.responses.keys()]).toEqual(['q1'])
  })

  it('starts fresh when the saved data is corrupt', () => {
    window.localStorage.setItem('quizette:quiz-progress:a1', '{"index":"x"')
    expect(loadQuizProgress('a1', ['q1'])).toEqual({ index: 0, responses: new Map() })
    window.localStorage.setItem('quizette:quiz-progress:a1', JSON.stringify({ index: -1, responses: { q1: ['a'] } }))
    expect(loadQuizProgress('a1', ['q1'])).toEqual({ index: 0, responses: new Map() })
  })
})
