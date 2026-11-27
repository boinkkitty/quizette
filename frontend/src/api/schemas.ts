import { z } from 'zod'

/** Runtime schemas mirroring the Go DTOs in backend/internal/adapters/http/dto.go. */

export const userSchema = z.object({
  id: z.string(),
  email: z.string(),
  name: z.string(),
  avatarUrl: z.string(),
})
export type User = z.infer<typeof userSchema>

export const providersSchema = z.object({ providers: z.array(z.enum(['google', 'github'])) })
export type OAuthProvider = z.infer<typeof providersSchema>['providers'][number]

export const topicSchema = z.object({
  id: z.string(),
  name: z.string(),
  chapterCount: z.number().int(),
  /** Topic-scoped attempts only; the scores are null until there is one. */
  attemptCount: z.number().int(),
  lastScorePct: z.number().nullable(),
  bestScorePct: z.number().nullable(),
  createdAt: z.string(),
})
export type Topic = z.infer<typeof topicSchema>

export const chapterStatusSchema = z.enum(['pending', 'ready', 'failed'])
export type ChapterStatus = z.infer<typeof chapterStatusSchema>

export const chapterSummarySchema = z.object({
  id: z.string(),
  topicId: z.string(),
  topicName: z.string(),
  title: z.string(),
  status: chapterStatusSchema,
  questionCount: z.number().int(),
  attemptCount: z.number().int(),
  bestScorePct: z.number().nullable(),
  createdAt: z.string(),
})
export type ChapterSummary = z.infer<typeof chapterSummarySchema>

export const topicDetailSchema = z.object({
  topic: topicSchema,
  chapters: z.array(chapterSummarySchema),
})
export type TopicDetail = z.infer<typeof topicDetailSchema>

export const jobStatusSchema = z.enum(['queued', 'running', 'succeeded', 'failed'])
export type JobStatus = z.infer<typeof jobStatusSchema>

export const jobSchema = z.object({
  id: z.string(),
  chapterId: z.string(),
  status: jobStatusSchema,
  error: z.string().optional(),
  requestedCount: z.number().int(),
  documentName: z.string(),
  documentSize: z.number().int(),
  attempts: z.number().int(),
  strategy: z.string().optional(),
  model: z.string().optional(),
  totalTokens: z.number().int(),
  totalMs: z.number().int(),
  queuedAt: z.string(),
  startedAt: z.string().nullable(),
  finishedAt: z.string().nullable(),
})
export type Job = z.infer<typeof jobSchema>

export const chapterSchema = z.object({
  id: z.string(),
  title: z.string(),
  status: chapterStatusSchema,
  topicId: z.string(),
  topicName: z.string(),
  questionCount: z.number().int(),
  latestJob: jobSchema.nullable(),
  createdAt: z.string(),
})
export type Chapter = z.infer<typeof chapterSchema>

export const questionTypeSchema = z.enum(['mcq', 'true_false', 'mrq'])
export type QuestionType = z.infer<typeof questionTypeSchema>

export const questionSchema = z.object({
  id: z.string(),
  position: z.number().int(),
  type: questionTypeSchema,
  prompt: z.string(),
  options: z.array(z.string()),
  correct: z.array(z.number().int()).optional(),
  explanation: z.string().optional(),
  sourceHint: z.string().optional(),
  /** The question's chapter, set in a quiz or attempt. */
  chapterId: z.string().optional(),
  chapterTitle: z.string().optional(),
  /** Set on a past attempt's question whose chapter is deleted. */
  chapterDeleted: z.boolean().optional(),
})
export type Question = z.infer<typeof questionSchema>

export const questionSetSchema = z.object({
  id: z.string(),
  chapterId: z.string(),
  questions: z.array(questionSchema),
})
export type QuestionSet = z.infer<typeof questionSetSchema>

/** What an attempt draws from (ADR 0010): one chapter, or a whole topic's ready chapters. */
export type ScopeRef = { chapterId: string } | { topicId: string }

const chapterScopeSchema = z.object({
  scope: z.literal('chapter'),
  chapterId: z.string(),
  chapterTitle: z.string(),
})
const topicScopeSchema = z.object({
  scope: z.literal('topic'),
  topicId: z.string(),
  topicName: z.string(),
})

/** POST /api/quizzes/draw: the questions of an unfinished quiz, without correct answers. */
export const drawnQuizSchema = z.discriminatedUnion('scope', [
  chapterScopeSchema.extend({ questions: z.array(questionSchema) }),
  topicScopeSchema.extend({ questions: z.array(questionSchema) }),
])
export type DrawnQuiz = z.infer<typeof drawnQuizSchema>

/** The scope a quiz or attempt draws from, as the draw and submit requests and /quizzes name it. */
export function scopeRef(
  quiz: { scope: 'topic'; topicId: string } | { scope: 'chapter'; chapterId: string },
): ScopeRef {
  return quiz.scope === 'topic' ? { topicId: quiz.topicId } : { chapterId: quiz.chapterId }
}

/** The quiz's title: the chapter, or "All of <topic>" for a whole-topic quiz. */
export function scopeTitle(quiz: DrawnQuiz): string {
  return quiz.scope === 'topic' ? `All of ${quiz.topicName}` : quiz.chapterTitle
}

/** An attempt sets chapterId for a chapter scope and topicId for a topic scope. */
export const attemptSummarySchema = z.object({
  id: z.string(),
  scope: z.enum(['chapter', 'topic']),
  chapterId: z.string().optional(),
  topicId: z.string().optional(),
  count: z.number().int(),
  totalScore: z.number(),
  maxScore: z.number().int(),
  scorePct: z.number(),
  startedAt: z.string(),
  submittedAt: z.string(),
})
export type AttemptSummary = z.infer<typeof attemptSummarySchema>

/** One row of GET /api/attempts: an attempt plus the names needed to list it across chapters. */
export const attemptHistoryItemSchema = attemptSummarySchema.extend({
  /** Empty for a topic quiz. topicId is set for both scopes. */
  chapterTitle: z.string().optional(),
  topicId: z.string(),
  topicName: z.string(),
  /** Whether the attempt's chapter (chapter quiz) or topic (topic quiz) is deleted. */
  scopeDeleted: z.boolean(),
  /** How many chapters the attempt drew questions from, deleted or not. */
  chapterCount: z.number().int(),
})
export type AttemptHistoryItem = z.infer<typeof attemptHistoryItemSchema>

export const answerSchema = z.object({
  questionId: z.string(),
  selected: z.array(z.number().int()),
  points: z.number(),
  correct: z.boolean(),
})
export type Answer = z.infer<typeof answerSchema>

export const attemptSchema = attemptSummarySchema.extend({
  /** Names a chapter scope; empty for a topic quiz. */
  chapterTitle: z.string().optional(),
  /** The topic quizzed, or the chapter's topic for a chapter quiz. */
  topicId: z.string(),
  topicName: z.string(),
  /** Whether the attempt's chapter (chapter quiz) or topic (topic quiz) is deleted. */
  scopeDeleted: z.boolean(),
  questions: z.array(questionSchema),
  answers: z.array(answerSchema),
})
export type Attempt = z.infer<typeof attemptSchema>

/** A past attempt's scope, narrowed so that a chapter scope always names its chapter. */
export type AttemptScope =
  | { scope: 'topic'; topicId: string; name: string }
  | { scope: 'chapter'; chapterId: string; name: string }

/**
 * The scope of a, named by its chapter's title or its topic's name. The API always
 * names a chapter scope's chapter, so a missing one throws instead of linking nowhere.
 */
export function attemptScope(a: AttemptHistoryItem | Attempt): AttemptScope {
  if (a.scope === 'topic') return { scope: 'topic', topicId: a.topicId, name: a.topicName }
  if (a.chapterId === undefined || a.chapterTitle === undefined) {
    throw new Error(`attempt ${a.id} has a chapter scope but no chapter`)
  }
  return { scope: 'chapter', chapterId: a.chapterId, name: a.chapterTitle }
}

/** The chapter a quiz or attempt question came from. */
export interface QuestionChapter {
  id: string
  title: string
  deleted: boolean
}

/**
 * The chapter q came from. The API names it on every question of a quiz or attempt,
 * so a missing one throws.
 */
export function questionChapter(q: Question): QuestionChapter {
  if (q.chapterId === undefined || q.chapterTitle === undefined) {
    throw new Error(`question ${q.id} has no chapter`)
  }
  return { id: q.chapterId, title: q.chapterTitle, deleted: q.chapterDeleted === true }
}

/** A submitted attempt in the Dashboard's Activity feed. chapterTitle is empty for a topic quiz. */
export const activityAttemptSchema = z.object({
  kind: z.literal('attempt'),
  id: z.string(),
  at: z.string(),
  scope: z.enum(['chapter', 'topic']),
  chapterId: z.string().optional(),
  chapterTitle: z.string(),
  topicId: z.string(),
  topicName: z.string(),
  scorePct: z.number(),
  count: z.number().int(),
})
export type ActivityAttempt = z.infer<typeof activityAttemptSchema>

/** A chapter upload in the Dashboard's Activity feed; id is the chapter's. */
export const activityUploadSchema = z.object({
  kind: z.literal('upload'),
  id: z.string(),
  at: z.string(),
  chapterTitle: z.string(),
  topicId: z.string(),
  topicName: z.string(),
  status: chapterStatusSchema,
  questionCount: z.number().int(),
})
export type ActivityUpload = z.infer<typeof activityUploadSchema>

export const activityItemSchema = z.discriminatedUnion('kind', [
  activityAttemptSchema,
  activityUploadSchema,
])
export type ActivityItem = z.infer<typeof activityItemSchema>

/** GET /api/dashboard. Chapter figures count chapter quizzes only (ADR 0010). */
export const dashboardSchema = z.object({
  chapterCount: z.number().int(),
  generatingCount: z.number().int(),
  failedCount: z.number().int(),
  stats: z.object({
    averageScorePct: z.number().nullable(),
    attemptCount: z.number().int(),
    quizzesThisWeek: z.number().int(),
    questionsAnswered: z.number().int(),
    chaptersQuizzed: z.number().int(),
    readyChapters: z.number().int(),
  }),
  topics: z.array(
    z.object({
      id: z.string(),
      name: z.string(),
      readyChapters: z.number().int(),
      chaptersQuizzed: z.number().int(),
      averageLastScorePct: z.number().nullable(),
    }),
  ),
  /** The newest 30 uploads and attempts, newest first. */
  activity: z.array(activityItemSchema),
})
export type Dashboard = z.infer<typeof dashboardSchema>

export const uploadResultSchema = z.object({
  jobId: z.string(),
  chapterId: z.string(),
  topicId: z.string(),
})
export type UploadResult = z.infer<typeof uploadResultSchema>

export const errorBodySchema = z.object({
  error: z.string(),
  message: z.string(),
  problems: z.array(z.string()).optional(),
})
