import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

import type {
  Attempt,
  AttemptHistoryItem,
  Chapter,
  ChapterSummary,
  DrawnQuiz,
  Job,
  Topic,
  User,
} from '../api/schemas'

export const user: User = { id: 'u1', email: 'ada@example.com', name: 'Ada', avatarUrl: '' }

export const unauthorized = () =>
  HttpResponse.json({ error: 'unauthorized', message: 'not logged in' }, { status: 401 })

export function topic(overrides: Partial<Topic> = {}): Topic {
  return {
    id: 't1',
    name: 'Biology',
    chapterCount: 1,
    attemptCount: 0,
    lastScorePct: null,
    bestScorePct: null,
    createdAt: '2026-11-20T10:00:00Z',
    ...overrides,
  }
}

export const topics: Topic[] = [topic()]

export function job(overrides: Partial<Job> = {}): Job {
  return {
    id: 'j1',
    chapterId: 'c1',
    status: 'queued',
    requestedCount: 10,
    documentName: 'cells.pdf',
    documentSize: 1024,
    attempts: 0,
    totalTokens: 0,
    totalMs: 0,
    queuedAt: '2026-11-20T10:00:00Z',
    startedAt: null,
    finishedAt: null,
    ...overrides,
  }
}

export function attempt(overrides: Partial<Attempt> = {}): Attempt {
  return {
    id: 'a1',
    scope: 'chapter',
    chapterId: 'c1',
    count: 5,
    totalScore: 0,
    maxScore: 5,
    scorePct: 0,
    startedAt: '2026-11-20T10:05:00Z',
    submittedAt: '2026-11-20T10:05:00Z',
    chapterTitle: 'Cells',
    topicId: 't1',
    topicName: 'Biology',
    scopeDeleted: false,
    answers: [],
    questions: [
      {
        id: 'q1',
        position: 0,
        chapterId: 'c1',
        chapterTitle: 'Cells',
        type: 'mcq',
        prompt: 'Powerhouse of the cell?',
        options: ['Nucleus', 'Mitochondria', 'Ribosome', 'Golgi'],
      },
      {
        id: 'q2',
        position: 1,
        chapterId: 'c1',
        chapterTitle: 'Cells',
        type: 'true_false',
        prompt: 'Plant cells have walls.',
        options: ['True', 'False'],
      },
      {
        id: 'q3',
        position: 2,
        chapterId: 'c1',
        chapterTitle: 'Cells',
        type: 'mrq',
        prompt: 'Which are organelles?',
        options: ['Nucleus', 'Water', 'Ribosome'],
      },
    ],
    ...overrides,
  }
}

/** A chapter quiz over attempt()'s questions, as POST /api/quizzes/draw returns it. */
export function drawnQuiz(
  overrides: Partial<Omit<Extract<DrawnQuiz, { scope: 'chapter' }>, 'scope'>> = {},
): DrawnQuiz {
  return {
    scope: 'chapter',
    chapterId: 'c1',
    chapterTitle: 'Cells',
    questions: attempt().questions,
    ...overrides,
  }
}

/** A whole-topic quiz over attempt()'s questions, as POST /api/quizzes/draw returns it. */
export function topicQuiz(
  overrides: Partial<Omit<Extract<DrawnQuiz, { scope: 'topic' }>, 'scope'>> = {},
): DrawnQuiz {
  return {
    scope: 'topic',
    topicId: 't1',
    topicName: 'Biology',
    questions: attempt().questions,
    ...overrides,
  }
}

export function chapter(overrides: Partial<Chapter> = {}): Chapter {
  return {
    id: 'c1',
    title: 'Cells',
    status: 'ready',
    topicId: 't1',
    topicName: 'Biology',
    questionCount: 10,
    latestJob: null,
    createdAt: '2026-11-20T10:00:00Z',
    ...overrides,
  }
}

export function chapterSummary(overrides: Partial<ChapterSummary> = {}): ChapterSummary {
  return {
    id: 'c1',
    topicId: 't1',
    topicName: 'Biology',
    title: 'Cells',
    status: 'ready',
    questionCount: 10,
    attemptCount: 2,
    bestScorePct: 80,
    createdAt: '2026-11-20T10:00:00Z',
    ...overrides,
  }
}

export function historyItem(overrides: Partial<AttemptHistoryItem> = {}): AttemptHistoryItem {
  return {
    id: 'a1',
    scope: 'chapter',
    chapterId: 'c1',
    count: 5,
    totalScore: 2.5,
    maxScore: 5,
    scorePct: 50,
    startedAt: '2026-11-20T10:00:00Z',
    submittedAt: '2026-11-20T10:05:00Z',
    chapterTitle: 'Cells',
    topicId: 't1',
    topicName: 'Biology',
    scopeDeleted: false,
    chapterCount: 1,
    ...overrides,
  }
}

/** Default happy-path handlers; tests override with server.use(...). */
export const handlers = [
  http.get('/api/auth/me', () => HttpResponse.json(user)),
  http.get('/api/auth/providers', () => HttpResponse.json({ providers: ['google', 'github'] })),
  http.get('/api/topics', () => HttpResponse.json(topics)),
]

export const server = setupServer(...handlers)
