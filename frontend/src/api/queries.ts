import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { z } from 'zod'

import { clearQuizFrom } from '../lib/quizSlot'
import { ApiError, request, requestVoid } from './client'
import {
  type Attempt,
  type ChapterStatus,
  type DrawnQuiz,
  drawnQuizSchema,
  type ScopeRef,
  scopeRef,
  attemptHistoryItemSchema,
  attemptSchema,
  attemptSummarySchema,
  chapterSchema,
  chapterSummarySchema,
  dashboardSchema,
  type Job,
  jobSchema,
  providersSchema,
  questionSetSchema,
  topicDetailSchema,
  topicSchema,
  uploadResultSchema,
  type User,
  userSchema,
} from './schemas'

export const keys = {
  me: ['me'] as const,
  providers: ['providers'] as const,
  topics: ['topics'] as const,
  topic: (id: string) => ['topics', id] as const,
  /** Prefix of every chapter-list query, for invalidation. */
  chapterLists: ['chapters', 'list'] as const,
  chapters: (filter: ChapterFilter = {}) => ['chapters', 'list', filter] as const,
  chapter: (id: string) => ['chapters', id] as const,
  questionSet: (id: string) => ['chapters', id, 'question-set'] as const,
  attempts: (chapterId: string) => ['chapters', chapterId, 'attempts'] as const,
  allAttempts: ['attempts'] as const,
  attempt: (id: string) => ['attempts', id] as const,
  job: (id: string) => ['jobs', id] as const,
  dashboard: ['dashboard'] as const,
}

/** Current user from the session cookie, or null when logged out. */
export const meQuery = queryOptions({
  queryKey: keys.me,
  queryFn: async ({ signal }): Promise<User | null> => {
    try {
      return await request(userSchema, '/auth/me', { signal })
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) return null
      throw err
    }
  },
  staleTime: 5 * 60_000,
})

export const providersQuery = queryOptions({
  queryKey: keys.providers,
  queryFn: ({ signal }) => request(providersSchema, '/auth/providers', { signal }),
  staleTime: Infinity,
})

export const topicsQuery = queryOptions({
  queryKey: keys.topics,
  queryFn: ({ signal }) => request(z.array(topicSchema), '/topics', { signal }),
})

const isProcessing = (status: string): boolean =>
  status === 'pending' || status === 'queued' || status === 'running'

export const topicQuery = (id: string) =>
  queryOptions({
    queryKey: keys.topic(id),
    queryFn: ({ signal }) => request(topicDetailSchema, `/topics/${id}`, { signal }),
    refetchInterval: (q) =>
      q.state.data?.chapters.some((c) => isProcessing(c.status)) === true ? 3000 : false,
  })

export interface ChapterFilter {
  status?: ChapterStatus
  topicId?: string
}

/** The user's chapters across topics, optionally narrowed by status and topic (the Quizzes pages). */
export const chaptersQuery = (filter: ChapterFilter = {}) =>
  queryOptions({
    queryKey: keys.chapters(filter),
    queryFn: ({ signal }) => {
      const params = new URLSearchParams()
      if (filter.status !== undefined) params.set('status', filter.status)
      if (filter.topicId !== undefined) params.set('topicId', filter.topicId)
      const qs = params.toString()
      return request(z.array(chapterSummarySchema), qs === '' ? '/chapters' : `/chapters?${qs}`, {
        signal,
      })
    },
    refetchInterval: (q) =>
      q.state.data?.some((c) => isProcessing(c.status)) === true ? 3000 : false,
  })

/** The user's most recent attempts across all chapters, newest first (the History page). */
export const allAttemptsQuery = queryOptions({
  queryKey: keys.allAttempts,
  queryFn: ({ signal }) => request(z.array(attemptHistoryItemSchema), '/attempts', { signal }),
})

/** The Dashboard's figures and Activity feed. Polls while a chapter is generating. */
export const dashboardQuery = queryOptions({
  queryKey: keys.dashboard,
  queryFn: ({ signal }) => request(dashboardSchema, '/dashboard', { signal }),
  refetchInterval: (q) => ((q.state.data?.generatingCount ?? 0) > 0 ? 3000 : false),
})

export const chapterQuery = (id: string) =>
  queryOptions({
    queryKey: keys.chapter(id),
    queryFn: ({ signal }) => request(chapterSchema, `/chapters/${id}`, { signal }),
    refetchInterval: (q) =>
      q.state.data !== undefined && isProcessing(q.state.data.status) ? 3000 : false,
  })

export const questionSetQuery = (chapterId: string) =>
  queryOptions({
    queryKey: keys.questionSet(chapterId),
    queryFn: ({ signal }) =>
      request(questionSetSchema, `/chapters/${chapterId}/question-set`, { signal }),
    staleTime: Infinity, // sets are immutable
  })

export const attemptsQuery = (chapterId: string) =>
  queryOptions({
    queryKey: keys.attempts(chapterId),
    queryFn: ({ signal }) =>
      request(z.array(attemptSummarySchema), `/chapters/${chapterId}/attempts`, { signal }),
  })

export const attemptQuery = (id: string) =>
  queryOptions({
    queryKey: keys.attempt(id),
    queryFn: ({ signal }) => request(attemptSchema, `/attempts/${id}`, { signal }),
  })

/** Polls a processing job until it finishes. */
export const jobQuery = (id: string) =>
  queryOptions({
    queryKey: keys.job(id),
    queryFn: ({ signal }) => request(jobSchema, `/jobs/${id}`, { signal }),
    refetchInterval: (q) =>
      q.state.data !== undefined && !isProcessing(q.state.data.status) ? false : 2000,
  })

export const isJobDone = (job: Job): boolean =>
  job.status === 'succeeded' || job.status === 'failed'

/**
 * useRefreshWhenJobDone refetches what a finished job changes (the chapter's status and question
 * count), so pages loaded while it ran, such as the Quizzes picker, don't keep it as pending.
 */
export function useRefreshWhenJobDone(job: Job | undefined, chapterId: string): void {
  const qc = useQueryClient()
  const done = job !== undefined && isJobDone(job)
  useEffect(() => {
    if (!done) return
    void qc.invalidateQueries({ queryKey: keys.topics })
    void qc.invalidateQueries({ queryKey: keys.dashboard })
    void qc.invalidateQueries({ queryKey: keys.chapterLists })
    void qc.invalidateQueries({ queryKey: keys.chapter(chapterId), exact: true })
  }, [done, chapterId, qc])
}

// ---- mutations ----

export interface Credentials {
  email: string
  password: string
  name?: string
}

export function useAuthMutation(mode: 'login' | 'signup') {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (c: Credentials) =>
      request(userSchema, `/auth/${mode}`, { method: 'POST', json: c }),
    onSuccess: (user) => {
      qc.setQueryData(keys.me, user)
    },
  })
}

export function useLogout() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: () => requestVoid('/auth/logout', { method: 'POST' }),
    // Forget cached data even if the server is unreachable.
    onSettled: () => {
      qc.clear()
      qc.setQueryData(keys.me, null)
    },
  })
}

export interface UploadInput {
  file: File
  count: number
  title: string
  topic: { kind: 'existing'; id: string } | { kind: 'new'; name: string }
}

export function useUpload() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: UploadInput) => {
      const form = new FormData()
      if (input.topic.kind === 'existing') form.set('topicId', input.topic.id)
      else form.set('newTopicName', input.topic.name)
      form.set('title', input.title)
      form.set('count', String(input.count))
      form.set('file', input.file) // file last: the server streams parts in order
      return request(uploadResultSchema, '/documents', { method: 'POST', form })
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: keys.topics })
      await qc.invalidateQueries({ queryKey: keys.dashboard })
      await qc.invalidateQueries({ queryKey: keys.chapterLists })
    },
  })
}

/** Draws a quiz from a chapter or a whole topic. Nothing is stored on the server until it is submitted. */
export function useDrawQuiz() {
  return useMutation({
    mutationFn: ({ scope, count }: { scope: ScopeRef; count: number }) =>
      request(drawnQuizSchema, '/quizzes/draw', { method: 'POST', json: { ...scope, count } }),
  })
}

/** Submits a finished quiz, which records it as an attempt and returns it graded. */
export function useSubmitQuiz(quiz: DrawnQuiz) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (answers: ReadonlyMap<string, readonly number[]>) =>
      request(attemptSchema, '/attempts', {
        method: 'POST',
        json: {
          ...scopeRef(quiz),
          questionIds: quiz.questions.map((q) => q.id),
          answers: [...answers].map(([questionId, selected]) => ({ questionId, selected })),
        },
      }),
    onSuccess: async (attempt: Attempt) => {
      qc.setQueryData(keys.attempt(attempt.id), attempt)
      // Chapter stats count chapter-scoped attempts only (ADR 0010).
      if (attempt.chapterId !== undefined) {
        await qc.invalidateQueries({ queryKey: keys.attempts(attempt.chapterId) })
        await qc.invalidateQueries({ queryKey: keys.chapter(attempt.chapterId) })
      }
      await qc.invalidateQueries({ queryKey: keys.topics })
      await qc.invalidateQueries({ queryKey: keys.dashboard })
      await qc.invalidateQueries({ queryKey: keys.chapterLists })
      await qc.invalidateQueries({ queryKey: keys.allAttempts, exact: true })
    },
  })
}

/** Deletes a chapter, and the tab's unfinished quiz if it drew from it. */
export function useDeleteChapter() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => requestVoid(`/chapters/${id}`, { method: 'DELETE' }),
    onSuccess: async (_, id) => {
      clearQuizFrom([id])
      await qc.invalidateQueries({ queryKey: keys.topics })
      await qc.invalidateQueries({ queryKey: keys.dashboard })
      await qc.invalidateQueries({ queryKey: keys.chapterLists })
      await qc.invalidateQueries({ queryKey: keys.allAttempts, exact: true })
    },
  })
}

/**
 * Deletes a topic, and the tab's unfinished quiz if it drew from one of chapterIds, the
 * topic's chapters.
 */
export function useDeleteTopic() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id }: { id: string; chapterIds: readonly string[] }) =>
      requestVoid(`/topics/${id}`, { method: 'DELETE' }),
    onSuccess: async (_, { chapterIds }) => {
      clearQuizFrom(chapterIds)
      await qc.invalidateQueries({ queryKey: keys.topics })
      await qc.invalidateQueries({ queryKey: keys.dashboard })
      await qc.invalidateQueries({ queryKey: keys.chapterLists })
      await qc.invalidateQueries({ queryKey: keys.allAttempts, exact: true })
    },
  })
}
