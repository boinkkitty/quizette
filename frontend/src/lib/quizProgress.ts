import { z } from 'zod'

/**
 * Saves an in-progress quiz (answers and current question) in localStorage, so a
 * reload or an accidentally closed tab doesn't lose it. Keyed by attempt, cleared once
 * the attempt is submitted. Storage failures (private mode, quota) are ignored: the
 * quiz still works, it just isn't restored.
 */
export interface QuizProgress {
  index: number
  responses: Map<string, number[]>
}

const key = (attemptId: string): string => `quizette:quiz-progress:${attemptId}`

const storedSchema = z.object({
  index: z.number().int().min(0),
  responses: z.record(z.string(), z.array(z.number().int().min(0))),
})

/** Returns the saved progress for questionIds, ignoring anything that doesn't fit them. */
export function loadQuizProgress(attemptId: string, questionIds: readonly string[]): QuizProgress {
  const empty: QuizProgress = { index: 0, responses: new Map() }
  try {
    const raw = window.localStorage.getItem(key(attemptId))
    if (raw === null) return empty
    const parsed = storedSchema.safeParse(JSON.parse(raw))
    if (!parsed.success) return empty
    const known = new Set(questionIds)
    const responses = new Map(Object.entries(parsed.data.responses).filter(([id]) => known.has(id)))
    return { index: Math.min(parsed.data.index, Math.max(questionIds.length - 1, 0)), responses }
  } catch {
    return empty
  }
}

export function saveQuizProgress(attemptId: string, progress: QuizProgress): void {
  try {
    window.localStorage.setItem(key(attemptId), JSON.stringify({ index: progress.index, responses: Object.fromEntries(progress.responses) }))
  } catch {
    // Not saved; the quiz keeps working.
  }
}

export function clearQuizProgress(attemptId: string): void {
  try {
    window.localStorage.removeItem(key(attemptId))
  } catch {
    // Nothing to clear.
  }
}
