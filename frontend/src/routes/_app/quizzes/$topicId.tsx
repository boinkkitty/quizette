import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link, notFound } from '@tanstack/react-router'
import { EyeIcon } from 'lucide-react'

import { Button } from '@/components/ui/button'

import { chaptersQuery, topicsQuery } from '../../../api/queries'
import { EmptyState } from '../../../components/EmptyState'
import { PageHeader } from '../../../components/PageHeader'
import { formatPct } from '../../../lib/format'

// Only ready chapters can be quizzed, so chapters still generating or failed are not fetched.
const readyChapters = (topicId: string) => chaptersQuery({ status: 'ready', topicId })

export const Route = createFileRoute('/_app/quizzes/$topicId')({
  loader: async ({ context, params }) => {
    // The topics list (names only) gives the page title without loading the topic's
    // unready chapters, and tells us when the topic doesn't exist or isn't the user's.
    const [topics] = await Promise.all([
      context.queryClient.ensureQueryData(topicsQuery),
      context.queryClient.ensureQueryData(readyChapters(params.topicId)),
    ])
    const topic = topics.find((t) => t.id === params.topicId)
    if (topic === undefined) throw notFound()
    return { topicName: topic.name }
  },
  component: QuizChaptersPage,
})

/** Step 2 of starting a quiz: pick a chapter in the chosen topic. */
function QuizChaptersPage() {
  const { topicId } = Route.useParams()
  const { topicName } = Route.useLoaderData()
  const { data: chapters } = useSuspenseQuery(readyChapters(topicId))
  return (
    <>
      <PageHeader
        title={topicName}
        subtitle={
          <Link to="/quizzes" className="hover:text-primary hover:underline">
            ← All quiz topics
          </Link>
        }
      />
      {chapters.length === 0 ? (
        <EmptyState title="No chapters ready in this topic">
          <Link
            to="/upload"
            search={{ topicId }}
            className="font-medium text-primary hover:underline"
          >
            Upload a PDF
          </Link>{' '}
          to add one.
        </EmptyState>
      ) : (
        <ul className="divide-y overflow-hidden rounded-xl border bg-card">
          {chapters.map((c) => (
            <li key={c.id} className="flex flex-wrap items-center justify-between gap-4 px-5 py-4">
              <div className="min-w-0">
                <Link
                  to="/chapters/$chapterId"
                  params={{ chapterId: c.id }}
                  className="font-medium hover:text-primary hover:underline"
                >
                  {c.title}
                </Link>
                <p className="mt-0.5 text-sm text-muted-foreground">
                  {c.questionCount} questions · {c.attemptCount} attempts
                  {c.bestScorePct !== null && ` · best ${formatPct(c.bestScorePct)}`}
                </p>
              </div>
              <div className="flex gap-2">
                <Button variant="outline" asChild>
                  <Link
                    to="/chapters/$chapterId/questions"
                    params={{ chapterId: c.id }}
                    aria-label={`View questions: ${c.title}`}
                  >
                    <EyeIcon aria-hidden /> View questions
                  </Link>
                </Button>
                <Button asChild>
                  <Link
                    to="/chapters/$chapterId"
                    params={{ chapterId: c.id }}
                    aria-label={`Start quiz: ${c.title}`}
                  >
                    Start quiz
                  </Link>
                </Button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </>
  )
}
