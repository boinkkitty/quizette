import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { chaptersQuery, topicsQuery } from '../../../api/queries'
import { PageHeader } from '../../../components/PageHeader'
import { QuizPicker } from '../../../components/QuizPicker'

// Every chapter, so step 2 can say how many aren't ready yet.
const allChapters = chaptersQuery()

/** chapterId or topicId preselects a scope and opens step 3 (Retake, Quiz this chapter). */
const searchSchema = z.object({
  chapterId: z.string().optional(),
  topicId: z.string().optional(),
})

/** The only place a quiz is set up: topic, then what to quiz, then how many. */
export const Route = createFileRoute('/_app/quizzes/')({
  validateSearch: searchSchema,
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.ensureQueryData(topicsQuery),
      context.queryClient.ensureQueryData(allChapters),
    ]),
  component: QuizzesPage,
})

function QuizzesPage() {
  const { data: topics } = useSuspenseQuery(topicsQuery)
  const { data: chapters } = useSuspenseQuery(allChapters)
  const search = Route.useSearch()
  return (
    <>
      <PageHeader title="Quizzes" subtitle="Quiz yourself on a whole topic or one chapter." />
      {/* Remount when the preselection changes, so the picker starts from the URL again. */}
      <QuizPicker
        key={`${search.chapterId ?? ''}:${search.topicId ?? ''}`}
        topics={topics}
        chapters={chapters}
        preselect={search}
      />
    </>
  )
}
