import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'

import { Badge } from '@/components/ui/badge'

import { allAttemptsQuery } from '../../api/queries'
import { EmptyState } from '../../components/EmptyState'
import { PageHeader } from '../../components/PageHeader'
import { formatDate, formatPct, formatPoints } from '../../lib/format'

export const Route = createFileRoute('/_app/history')({
  loader: ({ context }) => context.queryClient.ensureQueryData(allAttemptsQuery),
  component: HistoryPage,
})

function HistoryPage() {
  const { data: attempts } = useSuspenseQuery(allAttemptsQuery)
  return (
    <>
      <PageHeader title="History" subtitle="Your quiz attempts across all chapters, newest first." />
      {attempts.length === 0 ? (
        <EmptyState title="No attempts yet">
          Start one from{' '}
          <Link to="/quizzes" className="font-medium text-primary hover:underline">
            Quizzes
          </Link>
          .
        </EmptyState>
      ) : (
        <div className="overflow-x-auto rounded-xl border bg-card">
          <table className="w-full text-left text-sm">
            <thead className="border-b bg-muted text-xs text-muted-foreground uppercase">
              <tr>
                <th className="px-5 py-2 font-medium">Date</th>
                <th className="px-5 py-2 font-medium">Quiz</th>
                <th className="px-5 py-2 font-medium">Questions</th>
                <th className="px-5 py-2 font-medium">Score</th>
                <th className="px-5 py-2">
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {attempts.map((a) => (
                <tr key={a.id}>
                  <td className="px-5 py-3 whitespace-nowrap">{formatDate(a.submittedAt ?? a.startedAt)}</td>
                  <td className="px-5 py-3">
                    <Link to="/chapters/$chapterId" params={{ chapterId: a.chapterId }} className="font-medium hover:text-primary hover:underline">
                      {a.chapterTitle}
                    </Link>
                    <p className="text-xs text-muted-foreground">{a.topicName}</p>
                  </td>
                  <td className="px-5 py-3">{a.count}</td>
                  <td className="px-5 py-3 font-medium whitespace-nowrap">
                    {a.status === 'submitted' ? (
                      <>
                        {formatPoints(a.totalScore)} / {a.maxScore} <span className="text-muted-foreground">({formatPct(a.scorePct)})</span>
                      </>
                    ) : (
                      <Badge variant="info">In progress</Badge>
                    )}
                  </td>
                  <td className="px-5 py-3 text-right">
                    {a.status === 'submitted' ? (
                      <Link to="/attempts/$attemptId/results" params={{ attemptId: a.id }} className="font-medium text-primary hover:underline">
                        Review
                      </Link>
                    ) : (
                      <Link to="/attempts/$attemptId" params={{ attemptId: a.id }} className="font-medium text-primary hover:underline">
                        Resume
                      </Link>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}
