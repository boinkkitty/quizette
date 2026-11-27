import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { chaptersQuery, topicsQuery } from '../../api/queries'
import { PageHeader } from '../../components/PageHeader'
import { TopicGrid } from '../../components/TopicGrid'

// Every chapter, so each card counts its topic's chapters the way Quizzes does.
const allChapters = chaptersQuery()

/** Browse the library: every topic, each leading to its chapters. */
export const Route = createFileRoute('/_app/topics/')({
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.ensureQueryData(topicsQuery),
      context.queryClient.ensureQueryData(allChapters),
    ]),
  component: TopicsPage,
})

function TopicsPage() {
  const { data: topics } = useSuspenseQuery(topicsQuery)
  const { data: chapters } = useSuspenseQuery(allChapters)
  return (
    <>
      <PageHeader
        title="Topics"
        subtitle="Open a topic to see its chapters and quiz it, or upload a PDF to start a new one."
      />
      <TopicGrid topics={topics} chapters={chapters} />
    </>
  )
}
