import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { FileTextIcon, LayersIcon } from 'lucide-react'
import { useState } from 'react'

import { cn } from '@/lib/utils'

import { allAttemptsQuery } from '../../../api/queries'
import { attemptScope, type AttemptHistoryItem } from '../../../api/schemas'
import { Chip } from '../../../components/Chip'
import { EmptyState } from '../../../components/EmptyState'
import { PageHeader } from '../../../components/PageHeader'
import { ScorePill } from '../../../components/ScorePill'
import { formatDate } from '../../../lib/format'

export const Route = createFileRoute('/_app/history/')({
  loader: ({ context }) => context.queryClient.ensureQueryData(allAttemptsQuery),
  component: HistoryPage,
})

const filters = [
  ['all', 'All'],
  ['topic', 'Topic quizzes'],
  ['chapter', 'Chapter quizzes'],
] as const

type Filter = (typeof filters)[number][0]

function HistoryPage() {
  const { data: attempts } = useSuspenseQuery(allAttemptsQuery)
  const [filter, setFilter] = useState<Filter>('all')
  const shown = attempts.filter((a) => filter === 'all' || a.scope === filter)
  return (
    <>
      <PageHeader title="History" subtitle="Your quizzes, newest first." />
      {attempts.length === 0 ? (
        <EmptyState title="No quizzes yet">
          <Link to="/upload" className="font-medium text-primary hover:underline">
            Upload a PDF
          </Link>{' '}
          to make a chapter, then quiz yourself on it from{' '}
          <Link to="/quizzes" className="font-medium text-primary hover:underline">
            Quizzes
          </Link>
          .
        </EmptyState>
      ) : (
        <>
          <div role="group" aria-label="Show" className="mb-4 flex flex-wrap gap-2">
            {filters.map(([key, label]) => (
              <Chip key={key} active={filter === key} onClick={() => setFilter(key)}>
                {label}
              </Chip>
            ))}
          </div>
          <ul aria-label="Attempts" className="divide-y rounded-xl border bg-card">
            {shown.map((a) => (
              <li key={a.id}>
                <HistoryRow attempt={a} />
              </li>
            ))}
          </ul>
        </>
      )}
    </>
  )
}

function HistoryRow({ attempt: a }: { attempt: AttemptHistoryItem }) {
  const scope = attemptScope(a)
  const isTopicQuiz = scope.scope === 'topic'
  const deleted = a.scopeDeleted
  return (
    <Link
      to="/history/$attemptId"
      params={{ attemptId: a.id }}
      className="flex min-h-14 items-center gap-3 px-4 py-3 hover:bg-muted/50"
    >
      <span
        className={cn(
          'grid size-9 shrink-0 place-items-center rounded-lg',
          isTopicQuiz ? 'bg-secondary text-primary' : 'bg-muted',
          deleted && 'opacity-50',
        )}
      >
        {isTopicQuiz ? (
          <LayersIcon className="size-4" aria-hidden />
        ) : (
          <FileTextIcon className="size-4" aria-hidden />
        )}
      </span>
      <div className="min-w-0 flex-1">
        <p className={cn('truncate font-medium', deleted && 'text-muted-foreground')}>
          {isTopicQuiz ? `All of ${scope.name}` : scope.name}
          {deleted && <span className="font-normal"> (deleted)</span>}
        </p>
        <p className="truncate text-xs text-muted-foreground">
          {isTopicQuiz ? `${String(a.chapterCount)} chapters` : a.topicName} · {a.count} questions ·{' '}
          {formatDate(a.submittedAt)}
        </p>
      </div>
      <ScorePill pct={a.scorePct} />
    </Link>
  )
}
