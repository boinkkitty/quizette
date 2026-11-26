import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { CircleAlertIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import {
  attemptsQuery,
  chapterQuery,
  questionSetQuery,
  useDeleteChapter,
  useDrawQuiz,
} from '../../../api/queries'
import type { Chapter } from '../../../api/schemas'
import { ErrorMessage } from '../../../components/ErrorMessage'
import { JobProgress } from '../../../components/JobProgress'
import { QuestionReview } from '../../../components/QuestionReview'
import { StatusBadge } from '../../../components/StatusBadge'
import { ConfirmDialog } from '../../../components/ConfirmDialog'
import { EmptyState } from '../../../components/EmptyState'
import { Field } from '../../../components/Field'
import { PageHeader } from '../../../components/PageHeader'
import { Spinner } from '../../../components/Spinner'
import { countOptions, formatDate, formatPct, formatPoints } from '../../../lib/format'
import { startQuiz } from '../../../lib/quizSlot'

export const Route = createFileRoute('/_app/chapters/$chapterId/')({
  loader: ({ context, params }) =>
    context.queryClient.ensureQueryData(chapterQuery(params.chapterId)),
  component: ChapterPage,
})

function ChapterPage() {
  const { chapterId } = Route.useParams()
  const { data: chapter } = useSuspenseQuery(chapterQuery(chapterId))
  const del = useDeleteChapter()
  const navigate = useNavigate()

  return (
    <>
      <PageHeader
        title={chapter.title}
        subtitle={
          <span className="flex items-center gap-2">
            <Link
              to="/topics/$topicId"
              params={{ topicId: chapter.topicId }}
              className="hover:text-primary hover:underline"
            >
              ← {chapter.topicName}
            </Link>
            <StatusBadge status={chapter.status} />
          </span>
        }
        actions={
          <ConfirmDialog
            trigger={
              <Button variant="destructive" size="lg" disabled={del.isPending}>
                Delete chapter
              </Button>
            }
            title={`Delete "${chapter.title}"?`}
            description="This deletes its questions. Past attempts stay in History. It can't be undone."
            confirmLabel="Delete chapter"
            destructive
            onConfirm={() => {
              del.mutate(chapterId, {
                onSuccess: () => {
                  toast.success(`Deleted "${chapter.title}"`)
                  void navigate({ to: '/topics/$topicId', params: { topicId: chapter.topicId } })
                },
                onError: () => toast.error('Could not delete the chapter. Please try again.'),
              })
            }}
          />
        }
      />
      {chapter.status === 'pending' && chapter.latestJob !== null && (
        <JobProgress jobId={chapter.latestJob.id} chapterId={chapter.id} />
      )}
      {chapter.status === 'failed' && (
        <Alert variant="destructive">
          <CircleAlertIcon />
          <AlertTitle>Question generation failed</AlertTitle>
          <AlertDescription>
            <p>{chapter.latestJob?.error ?? 'Please try again.'}</p>
            <p>
              PDFs aren't stored, so{' '}
              <Link
                to="/upload"
                search={{ topicId: chapter.topicId }}
                className="font-medium underline"
              >
                re-upload the PDF
              </Link>{' '}
              to try again.
            </p>
          </AlertDescription>
        </Alert>
      )}
      {chapter.status === 'ready' && <ReadyChapter chapter={chapter} />}
    </>
  )
}

function ReadyChapter({ chapter }: { chapter: Chapter }) {
  const navigate = useNavigate()
  const start = useDrawQuiz(chapter.id)
  const options = countOptions(chapter.questionCount)
  const [count, setCount] = useState(Math.min(10, chapter.questionCount))
  const [reviewing, setReviewing] = useState(false)

  return (
    <div className="grid gap-6 lg:grid-cols-3">
      <Card className="h-fit lg:col-span-1">
        <CardHeader>
          <CardTitle>
            <h2 className="font-semibold">Start a quiz</h2>
          </CardTitle>
          <CardDescription>
            {chapter.questionCount} questions in this bank. Each quiz draws its questions at random.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Field label="Number of questions">
            {(c) => (
              <NativeSelect
                className="w-full"
                {...c}
                value={count}
                onChange={(e) => setCount(Number(e.target.value))}
              >
                {options.map((n) => (
                  <NativeSelectOption key={n} value={n}>
                    {n}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            )}
          </Field>
          {start.isError && <ErrorMessage error={start.error} title="Could not start quiz" />}
          <Button
            size="lg"
            className="w-full"
            disabled={start.isPending}
            onClick={() => {
              start.mutate(count, {
                onSuccess: (quiz) => {
                  startQuiz(quiz)
                  void navigate({ to: '/quizzes/run' })
                },
              })
            }}
          >
            {start.isPending && <Spinner />} Start quiz
          </Button>
        </CardContent>
      </Card>

      <div className="space-y-6 lg:col-span-2">
        <AttemptHistory chapterId={chapter.id} />
        <div>
          <Button variant="outline" size="lg" onClick={() => setReviewing((r) => !r)}>
            {reviewing ? 'Hide question bank' : 'Review question bank'}
          </Button>
          {reviewing && <QuestionBank chapterId={chapter.id} />}
        </div>
      </div>
    </div>
  )
}

function AttemptHistory({ chapterId }: { chapterId: string }) {
  const { data, error, isPending } = useQuery(attemptsQuery(chapterId))
  if (isPending) return <Spinner />
  if (error !== null) return <ErrorMessage error={error} />
  const attempts = data
  if (attempts.length === 0)
    return <EmptyState title="No attempts yet">Your quiz history will appear here.</EmptyState>
  return (
    <Card className="pb-0">
      <CardHeader>
        <CardTitle>
          <h2 className="font-semibold">Attempt history</h2>
        </CardTitle>
      </CardHeader>
      <table className="w-full text-left text-sm">
        <thead className="border-b bg-muted text-xs text-muted-foreground uppercase">
          <tr>
            <th className="px-6 py-2 font-medium">Date</th>
            <th className="px-6 py-2 font-medium">Questions</th>
            <th className="px-6 py-2 font-medium">Score</th>
            <th className="px-6 py-2" />
          </tr>
        </thead>
        <tbody className="divide-y">
          {attempts.map((a) => (
            <tr key={a.id}>
              <td className="px-6 py-3">{formatDate(a.submittedAt)}</td>
              <td className="px-6 py-3">{a.count}</td>
              <td className="px-6 py-3 font-medium">
                {formatPoints(a.totalScore)} / {a.maxScore}{' '}
                <span className="text-muted-foreground">({formatPct(a.scorePct)})</span>
              </td>
              <td className="px-6 py-3 text-right">
                <Link
                  to="/attempts/$attemptId/results"
                  params={{ attemptId: a.id }}
                  className="font-medium text-primary hover:underline"
                >
                  Review
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </Card>
  )
}

function QuestionBank({ chapterId }: { chapterId: string }) {
  const { data, error, isPending } = useQuery(questionSetQuery(chapterId))
  if (isPending) return <Spinner className="mt-4" />
  if (error !== null) return <ErrorMessage error={error} />
  return (
    <ol className="mt-4 space-y-4">
      {data.questions.map((q, i) => (
        <li key={q.id}>
          <QuestionReview index={i} question={q} />
        </li>
      ))}
    </ol>
  )
}
