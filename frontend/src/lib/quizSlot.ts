import { z } from 'zod'

import { type DrawnQuiz, drawnQuizSchema } from '../api/schemas'

/**
 * The tab's unfinished quiz (ADR 0011): the drawn questions, the answers so far and the
 * current question, in one sessionStorage slot. sessionStorage lasts as long as the tab,
 * so a reload keeps the quiz but closing the tab discards it, and nothing lingers on a
 * shared computer. Starting a quiz replaces the whole slot. Storage failures (private
 * mode, quota) are ignored: the quiz still works, it just isn't restored.
 */
export interface QuizProgress {
  index: number
  responses: Map<string, number[]>
}

export interface UnfinishedQuiz extends QuizProgress {
  quiz: DrawnQuiz
}

const key = 'quizette:quiz'

const storedSchema = z.object({
  quiz: drawnQuizSchema,
  index: z.number().int().min(0),
  responses: z.record(z.string(), z.array(z.number().int().min(0))),
})
type Stored = z.infer<typeof storedSchema>

function read(): Stored | null {
  try {
    const raw = window.sessionStorage.getItem(key)
    if (raw === null) return null
    const parsed = storedSchema.safeParse(JSON.parse(raw))
    return parsed.success ? parsed.data : null
  } catch {
    return null
  }
}

function write(stored: Stored): void {
  try {
    window.sessionStorage.setItem(key, JSON.stringify(stored))
  } catch {
    // Not saved; the quiz keeps working.
  }
}

/** Puts a freshly drawn quiz in the slot, discarding whatever was there. */
export function startQuiz(quiz: DrawnQuiz): void {
  write({ quiz, index: 0, responses: {} })
}

/** Returns the tab's unfinished quiz, or null if there is none. */
export function loadQuiz(): UnfinishedQuiz | null {
  const stored = read()
  if (stored === null) return null
  const { quiz } = stored
  const known = new Set(quiz.questions.map((q) => q.id))
  const responses = new Map(Object.entries(stored.responses).filter(([id]) => known.has(id)))
  const index = Math.min(stored.index, Math.max(quiz.questions.length - 1, 0))
  return { quiz, index, responses }
}

/** Saves the answers and position of the quiz in the slot. Does nothing if it is empty. */
export function saveQuizProgress(progress: QuizProgress): void {
  const stored = read()
  if (stored === null) return
  write({ ...stored, index: progress.index, responses: Object.fromEntries(progress.responses) })
}

export function clearQuiz(): void {
  try {
    window.sessionStorage.removeItem(key)
  } catch {
    // Nothing to clear.
  }
}
