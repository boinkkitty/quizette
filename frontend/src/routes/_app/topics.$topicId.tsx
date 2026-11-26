import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'

import { topicQuery, useDeleteTopic } from '../../api/queries'
import { StatusBadge } from '../../components/StatusBadge'
import { ConfirmDialog } from '../../components/ConfirmDialog'
import { EmptyState } from '../../components/EmptyState'
import { PageHeader } from '../../components/PageHeader'
import { formatPct } from '../../lib/format'

export const Route = createFileRoute('/_app/topics/$topicId')({
  loader: ({ context, params }) => context.queryClient.ensureQueryData(topicQuery(params.topicId)),
  component: TopicPage,
})

function TopicPage() {
  const { topicId } = Route.useParams()
  const { data } = useSuspenseQuery(topicQuery(topicId))
  const del = useDeleteTopic()
  const navigate = useNavigate()

  return (
    <>
      <PageHeader
        title={data.topic.name}
        subtitle={
          <Link to="/topics" className="hover:text-primary hover:underline">
            ← Topics
          </Link>
        }
        actions={
          <div className="flex gap-2">
            <ConfirmDialog
              trigger={
                <Button variant="destructive" size="lg" disabled={del.isPending}>
                  Delete topic
                </Button>
              }
              title={`Delete "${data.topic.name}"?`}
              description="This deletes all its chapters and questions. Past attempts stay in History. It can't be undone."
              confirmLabel="Delete topic"
              destructive
              onConfirm={() => {
                del.mutate(
                  { id: topicId, chapterIds: data.chapters.map((c) => c.id) },
                  {
                    onSuccess: () => {
                      toast.success(`Deleted "${data.topic.name}"`)
                      void navigate({ to: '/topics' })
                    },
                    onError: () => toast.error('Could not delete the topic. Please try again.'),
                  },
                )
              }}
            />
            <Button variant="outline" size="lg" asChild>
              <Link to="/upload" search={{ topicId }}>
                Add chapter
              </Link>
            </Button>
            <Button size="lg" asChild>
              <Link to="/quizzes" search={{ topicId }}>
                Quiz this topic
              </Link>
            </Button>
          </div>
        }
      />
      {data.chapters.length === 0 ? (
        <EmptyState title="No chapters in this topic yet">
          <Link
            to="/upload"
            search={{ topicId }}
            className="font-medium text-primary hover:underline"
          >
            Upload a PDF
          </Link>{' '}
          to add its first chapter.
        </EmptyState>
      ) : (
        <ul className="divide-y overflow-hidden rounded-xl border bg-card">
          {data.chapters.map((c) => (
            <li key={c.id}>
              <Link
                to="/chapters/$chapterId"
                params={{ chapterId: c.id }}
                className="flex items-center justify-between gap-4 px-5 py-4 transition-colors hover:bg-secondary/40"
              >
                <div>
                  <p className="font-medium">{c.title}</p>
                  <p className="mt-0.5 text-sm text-muted-foreground">
                    {c.status === 'ready'
                      ? `${String(c.questionCount)} questions · ${String(c.attemptCount)} attempts`
                      : ' '}
                    {c.bestScorePct !== null && ` · best ${formatPct(c.bestScorePct)}`}
                  </p>
                </div>
                <StatusBadge status={c.status} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </>
  )
}
