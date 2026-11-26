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

/** The scope a quiz was drawn from, as the draw and submit requests name it. */
export function scopeRef(quiz: DrawnQuiz): ScopeRef {
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
  /** chapterTitle names a chapter scope, topicName a topic scope. */
  chapterTitle: z.string().optional(),
  topicName: z.string().optional(),
  questions: z.array(questionSchema),
  answers: z.array(answerSchema),
})
export type Attempt = z.infer<typeof attemptSchema>

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
