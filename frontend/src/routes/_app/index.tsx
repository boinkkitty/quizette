import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'

import { topicsQuery } from '../../api/queries'
import { PageHeader } from '../../components/PageHeader'
import { TopicGrid } from '../../components/TopicGrid'

// The dashboard. Until it has its own content it shows the topics, as /topics does.
export const Route = createFileRoute('/_app/')({
  loader: ({ context }) => context.queryClient.ensureQueryData(topicsQuery),
  component: DashboardPage,
})

function DashboardPage() {
  const { data: topics } = useSuspenseQuery(topicsQuery)
  return (
    <>
      <PageHeader
        title="Your topics"
        subtitle="Each topic holds chapters; each chapter is a question bank generated from one PDF."
      />
      <TopicGrid topics={topics} />
    </>
  )
}
