import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link, redirect } from '@tanstack/react-router'

import { Card, CardContent } from '@/components/ui/card'

import { attemptQuery } from '../../../api/queries'
import { QuestionReview } from '../../../components/QuestionReview'
import { PageHeader } from '../../../components/PageHeader'
import { formatPct, formatPoints } from '../../../lib/format'

export const Route = createFileRoute('/_app/attempts/$attemptId/results')({
  loader: async ({ context, params }) => {
    const attempt = await context.queryClient.ensureQueryData(attemptQuery(params.attemptId))
    if (attempt.status !== 'submitted') throw redirect({ to: '/attempts/$attemptId', params })
  },
  component: ResultsPage,
})

function ResultsPage() {
  const { attemptId } = Route.useParams()
  const { data: attempt } = useSuspenseQuery(attemptQuery(attemptId))
  const answers = new Map(attempt.answers.map((a) => [a.questionId, a]))
  const correctCount = attempt.answers.filter((a) => a.correct).length

  return (
    <div className="mx-auto max-w-2xl">
      <PageHeader
        title="Results"
        subtitle={
          <Link
            to="/chapters/$chapterId"
            params={{ chapterId: attempt.chapterId }}
            className="hover:text-primary hover:underline"
          >
            ← {attempt.chapterTitle}
          </Link>
        }
      />
      <Card className="mb-6 border-primary/20 bg-secondary/40">
        <CardContent className="flex items-center justify-between">
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
      <ol className="space-y-4">
        {attempt.questions.map((q, i) => (
          <li key={q.id}>
            <QuestionReview index={i} question={q} answer={answers.get(q.id)} />
          </li>
        ))}
      </ol>
    </div>
  )
}
