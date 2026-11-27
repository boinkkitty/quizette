import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'

import type { ChapterSummary } from '../../api/schemas'
import { topicQuery, useDeleteChapter, useDeleteTopic } from '../../api/queries'
import { StatusBadge } from '../../components/StatusBadge'
import { DeleteAction } from '../../components/DeleteAction'
import { EmptyState } from '../../components/EmptyState'
import { backLinkClass, PageHeader } from '../../components/PageHeader'
import { formatPct, plural } from '../../lib/format'

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
          <Link to="/topics" className={backLinkClass}>
            ← Topics
          </Link>
        }
        actions={
          <div className="flex flex-wrap gap-2">
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
            <DeleteAction
              label="Delete topic"
              disabled={del.isPending}
              title={`Delete "${data.topic.name}"?`}
              description="This deletes all its chapters and questions. Past attempts stay in History. It can't be undone."
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
            <li
              key={c.id}
              className="relative flex items-center justify-between gap-4 px-5 py-4 transition-colors hover:bg-secondary/40"
            >
              <div className="min-w-0">
                {/* Stretched over the row, so the whole row opens the chapter. */}
                <Link
                  to="/chapters/$chapterId"
                  params={{ chapterId: c.id }}
                  className="font-medium break-words after:absolute after:inset-0"
                >
                  {c.title}
                </Link>
                {c.status === 'failed' ? (
                  <Link
                    to="/upload"
                    search={{ topicId }}
                    className="relative mt-0.5 block text-sm text-destructive hover:underline"
                  >
                    Generation failed: re-upload
                  </Link>
                ) : (
                  <p className="mt-0.5 text-sm text-muted-foreground">
                    {c.status === 'ready'
                      ? `${plural(c.questionCount, 'question')} · ${plural(c.attemptCount, 'attempt')}`
                      : 'Generating questions…'}
                    {c.status === 'ready' &&
                      c.bestScorePct !== null &&
                      ` · best ${formatPct(c.bestScorePct)}`}
                  </p>
                )}
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <StatusBadge status={c.status} />
                <DeleteChapter chapter={c} />
              </div>
            </li>
          ))}
        </ul>
      )}
    </>
  )
}

/** A chapter row's bin; the row stays on this page, which refetches without the chapter. */
function DeleteChapter({ chapter }: { chapter: ChapterSummary }) {
  const del = useDeleteChapter()
  return (
    <DeleteAction
      label="Delete chapter"
      name={chapter.title}
      // Above the row's stretched link, so a click deletes rather than opens the chapter.
      className="relative"
      disabled={del.isPending}
      title={`Delete "${chapter.title}"?`}
      description="This deletes its questions. Past attempts stay in History. It can't be undone."
      onConfirm={() => {
        del.mutate(chapter.id, {
          onSuccess: () => toast.success(`Deleted "${chapter.title}"`),
          onError: () => toast.error('Could not delete the chapter. Please try again.'),
        })
      }}
    />
  )
}
