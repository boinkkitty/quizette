import { Link } from '@tanstack/react-router'
import { FolderIcon } from 'lucide-react'
import { toast } from 'sonner'

import { useDeleteTopic } from '../api/queries'
import type { ChapterSummary, Topic } from '../api/schemas'
import { chapterCountLabel } from '../lib/format'
import { DeleteAction } from './DeleteAction'
import { EmptyState } from './EmptyState'

/**
 * The user's topics as cards linking to each topic page, each with a bin to delete it; empty, it
 * points to Upload.
 */
export function TopicGrid({
  topics,
  chapters,
}: {
  topics: readonly Topic[]
  /** Every chapter of every topic, to count each topic's chapters by status. */
  chapters: readonly ChapterSummary[]
}) {
  if (topics.length === 0) {
    return (
      <EmptyState title="No topics yet">
        <Link to="/upload" className="font-medium text-primary hover:underline">
          Upload your first PDF
        </Link>{' '}
        to generate a question bank.
      </EmptyState>
    )
  }
  return (
    <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {topics.map((t) => {
        const mine = chapters.filter((c) => c.topicId === t.id)
        return (
          <li key={t.id} className="relative">
            <Link
              to="/topics/$topicId"
              params={{ topicId: t.id }}
              className="group flex h-full items-start gap-3 rounded-xl border bg-card p-5 pr-14 transition-colors hover:border-primary hover:bg-secondary/40"
            >
              <span className="rounded-lg bg-secondary p-2 text-primary">
                <FolderIcon className="size-5" aria-hidden />
              </span>
              <span className="min-w-0">
                <span className="block font-medium break-words group-hover:text-primary">
                  {t.name}
                </span>
                <span className="mt-1 block text-sm text-muted-foreground">
                  {chapterCountLabel(mine)}
                </span>
              </span>
            </Link>
            <DeleteTopic topic={t} chapterIds={mine.map((c) => c.id)} />
          </li>
        )
      })}
    </ul>
  )
}

/** A topic card's bin, in its top-right corner; Topics refetches without the topic. */
function DeleteTopic({ topic, chapterIds }: { topic: Topic; chapterIds: readonly string[] }) {
  const del = useDeleteTopic()
  return (
    <DeleteAction
      label="Delete topic"
      name={topic.name}
      className="absolute top-2 right-2"
      disabled={del.isPending}
      title={`Delete "${topic.name}"?`}
      description="This deletes all its chapters and questions. Past attempts stay in History. It can't be undone."
      onConfirm={() => {
        del.mutate(
          { id: topic.id, chapterIds },
          {
            onSuccess: () => toast.success(`Deleted "${topic.name}"`),
            onError: () => toast.error('Could not delete the topic. Please try again.'),
          },
        )
      }}
    />
  )
}
