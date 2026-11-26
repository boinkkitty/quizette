import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { topicsQuery } from '../../api/queries'
import { PageHeader } from '../../components/PageHeader'
import { TopicGrid } from '../../components/TopicGrid'

/** Browse the library: every topic, each leading to its chapters. */
export const Route = createFileRoute('/_app/topics/')({
  loader: ({ context }) => context.queryClient.ensureQueryData(topicsQuery),
  component: TopicsPage,
})

function TopicsPage() {
  const { data: topics } = useSuspenseQuery(topicsQuery)
  return (
    <>
      <PageHeader
        title="Topics"
        subtitle="Each topic holds chapters; each chapter is a question bank generated from one PDF."
      />
      <TopicGrid topics={topics} />
    </>
  )
}
