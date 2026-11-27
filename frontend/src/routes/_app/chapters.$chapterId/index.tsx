import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { CircleAlertIcon } from 'lucide-react'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardHeader, CardTitle } from '@/components/ui/card'

import { attemptsQuery, chapterQuery, useDeleteChapter } from '../../../api/queries'
import { ErrorMessage } from '../../../components/ErrorMessage'
import { JobProgress } from '../../../components/JobProgress'
import { StatusBadge } from '../../../components/StatusBadge'
import { DeleteAction } from '../../../components/DeleteAction'
import { EmptyState } from '../../../components/EmptyState'
import { backLinkClass, PageHeader } from '../../../components/PageHeader'
import { Spinner } from '../../../components/Spinner'
import { formatDate, formatPct, formatPoints } from '../../../lib/format'

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
          <span className="flex flex-wrap items-center gap-x-2">
            <Link
              to="/topics/$topicId"
              params={{ topicId: chapter.topicId }}
              className={backLinkClass}
            >
              ← {chapter.topicName}
            </Link>
            <StatusBadge status={chapter.status} />
          </span>
        }
        actions={
          <div className="flex flex-wrap gap-2">
            {chapter.status === 'ready' && (
              <>
                <Button variant="outline" size="lg" asChild>
                  <Link to="/chapters/$chapterId/questions" params={{ chapterId }}>
                    View questions
                  </Link>
                </Button>
                <Button size="lg" asChild>
                  <Link to="/quizzes" search={{ chapterId }}>
                    Quiz this chapter
                  </Link>
                </Button>
              </>
            )}
            <DeleteAction
              label="Delete chapter"
              disabled={del.isPending}
              title={`Delete "${chapter.title}"?`}
              description="This deletes its questions. Past attempts stay in History. It can't be undone."
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
          </div>
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
      {chapter.status === 'ready' && <AttemptHistory chapterId={chapter.id} />}
    </>
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
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead className="border-b bg-muted text-xs text-muted-foreground uppercase">
            <tr>
              <th className="px-4 py-2 sm:px-6 font-medium">Date</th>
              <th className="px-4 py-2 sm:px-6 font-medium">Questions</th>
              <th className="px-4 py-2 sm:px-6 font-medium">Score</th>
              <th className="px-4 py-2 sm:px-6" />
            </tr>
          </thead>
          <tbody className="divide-y">
            {attempts.map((a) => (
              <tr key={a.id}>
                <td className="px-4 py-1.5 sm:px-6">{formatDate(a.submittedAt)}</td>
                <td className="px-4 py-1.5 sm:px-6">{a.count}</td>
                <td className="px-4 py-1.5 sm:px-6 font-medium">
                  {formatPoints(a.totalScore)} / {a.maxScore}{' '}
                  <span className="text-muted-foreground">({formatPct(a.scorePct)})</span>
                </td>
                <td className="px-4 py-1.5 sm:px-6 text-right">
                  <Link
                    to="/history/$attemptId"
                    params={{ attemptId: a.id }}
                    className="inline-flex min-h-11 items-center font-medium text-primary hover:underline"
                  >
                    Review
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  )
}
