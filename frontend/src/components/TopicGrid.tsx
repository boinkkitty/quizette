import { Link } from '@tanstack/react-router'
import { FolderIcon } from 'lucide-react'

import type { ChapterSummary, Topic } from '../api/schemas'
import { chapterCountLabel } from '../lib/format'
import { EmptyState } from './EmptyState'

/** The user's topics as cards linking to each topic page; empty, it points to Upload. */
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
      {topics.map((t) => (
        <li key={t.id}>
          <Link
            to="/topics/$topicId"
            params={{ topicId: t.id }}
            className="group flex items-start gap-3 rounded-xl border bg-card p-5 transition-colors hover:border-primary hover:bg-secondary/40"
          >
            <span className="rounded-lg bg-secondary p-2 text-primary">
              <FolderIcon className="size-5" aria-hidden />
            </span>
            <span>
              <p className="font-medium group-hover:text-primary">{t.name}</p>
              <p className="mt-1 text-sm text-muted-foreground">
                {chapterCountLabel(chapters.filter((c) => c.topicId === t.id))}
              </p>
            </span>
          </Link>
        </li>
      ))}
    </ul>
  )
}
