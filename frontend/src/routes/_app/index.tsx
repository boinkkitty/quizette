import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { FolderIcon } from 'lucide-react'

import { Button } from '@/components/ui/button'

import { topicsQuery } from '../../api/queries'
import { EmptyState } from '../../components/EmptyState'
import { PageHeader } from '../../components/PageHeader'

export const Route = createFileRoute('/_app/')({
  loader: ({ context }) => context.queryClient.ensureQueryData(topicsQuery),
  component: TopicsPage,
})

function TopicsPage() {
  const { data: topics } = useSuspenseQuery(topicsQuery)
  return (
    <>
      <PageHeader
        title="Your topics"
        subtitle="Each topic holds chapters; each chapter is a question bank generated from one PDF."
        actions={
          <Button size="lg" asChild>
            <Link to="/upload">Upload PDF</Link>
          </Button>
        }
      />
      {topics.length === 0 ? (
        <EmptyState title="No topics yet">
          <Link to="/upload" className="font-medium text-primary hover:underline">
            Upload your first PDF
          </Link>{' '}
          to generate a question bank.
        </EmptyState>
      ) : (
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
                    {t.chapterCount} {t.chapterCount === 1 ? 'chapter' : 'chapters'}
                  </p>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </>
  )
}
