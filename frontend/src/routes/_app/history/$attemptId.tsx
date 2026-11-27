import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { HistoryIcon, RotateCcwIcon, Trash2Icon } from 'lucide-react'
import { useState, type ComponentProps } from 'react'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'

import { attemptQuery } from '../../../api/queries'
import {
  attemptScope,
  questionChapter,
  scopeRef,
  type Answer,
  type Attempt,
  type AttemptScope,
  type Question,
  type QuestionChapter,
} from '../../../api/schemas'
import { Chip } from '../../../components/Chip'
import { PageHeader } from '../../../components/PageHeader'
import { QuestionReview } from '../../../components/QuestionReview'
import { formatDate, formatPct, formatPoints, plural } from '../../../lib/format'

export const Route = createFileRoute('/_app/history/$attemptId')({
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(attemptQuery(params.attemptId)),
  component: ResultsPage,
})

/** A chapter an attempt drew from, with its share of the score. */
interface ChapterScore extends QuestionChapter {
  pct: number
}

/**
 * The chapters the attempt's questions came from, in the order they first appear.
 * Each question is worth one point, so a chapter's percentage is its points over its
 * question count.
 */
function chapterScores(a: Attempt, answers: ReadonlyMap<string, Answer>): ChapterScore[] {
  const byChapter = new Map<string, { chapter: QuestionChapter; points: number; count: number }>()
  for (const q of a.questions) {
    const chapter = questionChapter(q)
    const c = byChapter.get(chapter.id) ?? { chapter, points: 0, count: 0 }
    c.points += answers.get(q.id)?.points ?? 0
    c.count++
    byChapter.set(chapter.id, c)
  }
  return [...byChapter.values()].map((c) => ({ ...c.chapter, pct: (c.points / c.count) * 100 }))
}

function ResultsPage() {
  const { attemptId } = Route.useParams()
  const { data: attempt } = useSuspenseQuery(attemptQuery(attemptId))
  const answers = new Map(attempt.answers.map((a) => [a.questionId, a]))
  const chapters = chapterScores(attempt, answers)
  const correctCount = attempt.answers.filter((a) => a.correct).length
  const scope = attemptScope(attempt)

  return (
    <div className="mx-auto max-w-2xl pb-24">
      <Link
        to="/history"
        className="mb-2 inline-flex min-h-11 items-center text-sm text-muted-foreground hover:text-primary"
      >
        ← History
      </Link>
      <PageHeader
        title={scope.name}
        subtitle={`${
          scope.scope === 'topic'
            ? `Whole-topic quiz · ${plural(chapters.length, 'chapter')}`
            : `Chapter quiz · ${attempt.topicName}`
        } · ${formatDate(attempt.submittedAt)}`}
      />
      <Card className="mb-6 border-primary/20 bg-secondary/40">
        <CardContent className="flex items-center justify-between gap-4">
          <div>
            <p className="text-sm text-muted-foreground">Score</p>
            <p className="text-3xl font-semibold" data-testid="score">
              {formatPoints(attempt.totalScore)} / {attempt.maxScore}
            </p>
            <p className="text-sm text-muted-foreground">
              {correctCount} fully correct · select-all questions earn partial credit
            </p>
          </div>
          <p className="text-4xl font-semibold text-primary">{formatPct(attempt.scorePct)}</p>
        </CardContent>
      </Card>
      <AnswersReview attempt={attempt} answers={answers} chapters={chapters} />
      <ResultsBar scope={scope} deleted={attempt.scopeDeleted} />
    </div>
  )
}

/**
 * The graded answers. A topic quiz gets one chip per chapter to filter them by, and
 * names each question's chapter while all are shown. Mistakes are answers that are
 * not fully correct, partial credit included. Questions keep their numbers from the
 * quiz whatever the filter.
 */
function AnswersReview({
  attempt,
  answers,
  chapters,
}: {
  attempt: Attempt
  answers: ReadonlyMap<string, Answer>
  chapters: readonly ChapterScore[]
}) {
  const [chapterId, setChapterId] = useState<string | null>(null)
  const [mistakesOnly, setMistakesOnly] = useState(false)
  const isTopicQuiz = attempt.scope === 'topic'
  const chosen = chapters.find((c) => c.id === chapterId)
  const shown = attempt.questions
    .map((question, index) => ({ question, index }))
    .filter(({ question }) => chosen === undefined || question.chapterId === chosen.id)
    .filter(({ question }) => !mistakesOnly || answers.get(question.id)?.correct !== true)

  return (
    <>
      {isTopicQuiz && (
        <div
          role="group"
          aria-label="Chapters"
          className="-mx-4 mb-3 flex gap-2 overflow-x-auto px-4 pb-1 md:mx-0 md:flex-wrap md:px-0"
        >
          <Chip active={chosen === undefined} onClick={() => setChapterId(null)}>
            All chapters
          </Chip>
          {chapters.map((c) => (
            <Chip
              key={c.id}
              active={chosen?.id === c.id}
              deleted={c.deleted}
              onClick={() => setChapterId(c.id)}
            >
              {c.deleted && (
                <>
                  <Trash2Icon className="size-3.5" aria-hidden />
                  <span className="sr-only">Deleted chapter: </span>
                </>
              )}
              <span className="max-w-40 truncate">{c.title}</span>
              <span className="font-semibold">{formatPct(c.pct)}</span>
            </Chip>
          ))}
        </div>
      )}
      <label className="mb-4 flex min-h-11 w-fit items-center gap-2 text-sm">
        <Checkbox
          checked={mistakesOnly}
          onCheckedChange={(checked) => setMistakesOnly(checked === true)}
        />
        Only show mistakes
      </label>
      {chosen !== undefined && (
        <div className="mb-4 flex min-h-11 items-center justify-between gap-3 rounded-lg bg-muted px-3 py-2 text-sm">
          {chosen.deleted ? (
            <span className="text-muted-foreground">
              {chosen.title} was deleted. Its questions stay here for this result.
            </span>
          ) : (
            <>
              <span className="min-w-0 truncate">{chosen.title}</span>
              <Link
                to="/chapters/$chapterId"
                params={{ chapterId: chosen.id }}
                className="inline-flex min-h-11 shrink-0 items-center font-medium text-primary hover:underline"
              >
                Open chapter →
              </Link>
            </>
          )}
        </div>
      )}
      <ol aria-label="Answers" className="space-y-4">
        {shown.map(({ question, index }) => (
          <li key={question.id}>
            {isTopicQuiz && chosen === undefined && <SourceLine question={question} />}
            <QuestionReview index={index} question={question} answer={answers.get(question.id)} />
          </li>
        ))}
      </ol>
      {shown.length === 0 && <p className="text-sm text-muted-foreground">No mistakes to show.</p>}
    </>
  )
}

/** Where a topic quiz's question came from, written above its card. */
function SourceLine({ question }: { question: Question }) {
  const chapter = questionChapter(question)
  if (chapter.deleted) {
    return (
      <p data-source className="mb-1.5 flex items-center gap-1.5 text-xs text-muted-foreground">
        <Trash2Icon className="size-3.5" aria-hidden />
        From a deleted chapter: {chapter.title}
      </p>
    )
  }
  return (
    <p data-source className="mb-1.5 text-xs text-muted-foreground">
      From{' '}
      <Link
        to="/chapters/$chapterId"
        params={{ chapterId: chapter.id }}
        className="font-medium hover:text-primary hover:underline"
      >
        {chapter.title}
      </Link>
    </p>
  )
}

/**
 * Retake · Go to <scope> · History, fixed to the bottom of the window. Once the scope
 * is deleted there is nothing to retake or go to, so only History stays. A topic quiz
 * with only some of its chapters deleted can still be retaken.
 */
function ResultsBar({ scope, deleted }: { scope: AttemptScope; deleted: boolean }) {
  return (
    <nav
      aria-label="Result actions"
      // md:left-60 clears the sidebar, which is w-60 in AppSidebar.
      className="fixed inset-x-0 bottom-0 z-10 border-t bg-background/95 backdrop-blur md:left-60"
    >
      <div className="mx-auto flex max-w-2xl items-center gap-2 px-4 py-3">
        {deleted ? (
          <p className="flex-1 text-xs text-muted-foreground">
            This {scope.scope} was deleted, so it can't be retaken.
          </p>
        ) : (
          <>
            <Button className="min-h-11 flex-1 sm:flex-none" asChild>
              <Link to="/quizzes" search={scopeRef(scope)}>
                <RotateCcwIcon aria-hidden /> Retake
              </Link>
            </Button>
            <Button variant="outline" className="min-h-11 min-w-0 flex-1 sm:flex-none" asChild>
              <ScopeLink scope={scope}>
                <span className="truncate">Go to {scope.name}</span>
              </ScopeLink>
            </Button>
          </>
        )}
        <Button variant="ghost" className="min-h-11" asChild>
          <Link to="/history">
            <HistoryIcon aria-hidden /> History
          </Link>
        </Button>
      </div>
    </nav>
  )
}

/** A link to the scope's topic or chapter page. Props pass through so Button's asChild can style it. */
function ScopeLink({
  scope,
  ...props
}: { scope: AttemptScope } & Omit<ComponentProps<typeof Link>, 'to' | 'params' | 'search'>) {
  return scope.scope === 'topic' ? (
    <Link {...props} to="/topics/$topicId" params={{ topicId: scope.topicId }} />
  ) : (
    <Link {...props} to="/chapters/$chapterId" params={{ chapterId: scope.chapterId }} />
  )
}
