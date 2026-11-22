import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { FolderIcon } from 'lucide-react'

import { chaptersQuery } from '../../../api/queries'
import type { ChapterSummary } from '../../../api/schemas'
import { EmptyState } from '../../../components/EmptyState'
import { PageHeader } from '../../../components/PageHeader'

export const Route = createFileRoute('/_app/quizzes/')({
  loader: ({ context }) => context.queryClient.ensureQueryData(readyChapters),
  component: QuizTopicsPage,
})

interface TopicQuizzes {
  topicId: string
  topicName: string
  chapters: number
}

/** Groups ready chapters into one entry per topic, keeping the API's order (topic name). */
function topicsWithQuizzes(chapters: readonly ChapterSummary[]): TopicQuizzes[] {
  const groups = new Map<string, TopicQuizzes>()
  for (const c of chapters) {
    const g = groups.get(c.topicId) ?? { topicId: c.topicId, topicName: c.topicName, chapters: 0 }
    g.chapters++
    groups.set(c.topicId, g)
  }
  return [...groups.values()]
}

// Only ready chapters can be quizzed, so chapters still generating or failed are not fetched.
const readyChapters = chaptersQuery({ status: 'ready' })

/** Step 1 of starting a quiz: pick a topic. */
function QuizTopicsPage() {
  const { data: chapters } = useSuspenseQuery(readyChapters)
  const topics = topicsWithQuizzes(chapters)
  return (
    <>
      <PageHeader title="Quizzes" subtitle="Choose a topic, then pick a chapter to practise." />
      {topics.length === 0 ? (
        <EmptyState title="No quizzes yet">
          <Link to="/upload" className="font-medium text-primary hover:underline">
            Upload a PDF
          </Link>{' '}
          to generate your first question bank.
        </EmptyState>
      ) : (
        <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {topics.map((t) => (
            <li key={t.topicId}>
              <Link
                to="/quizzes/$topicId"
                params={{ topicId: t.topicId }}
                className="group flex h-full items-start gap-3 rounded-xl border bg-card p-5 transition-colors hover:border-primary hover:bg-secondary/40"
              >
                <span className="rounded-lg bg-secondary p-2 text-primary">
                  <FolderIcon className="size-5" aria-hidden />
                </span>
                <span>
                  <span className="block font-medium group-hover:text-primary">{t.topicName}</span>
                  <span className="mt-1 block text-sm text-muted-foreground">
                    {t.chapters} {t.chapters === 1 ? 'chapter' : 'chapters'}
                  </span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </>
  )
}
