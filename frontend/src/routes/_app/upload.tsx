import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

import { jobQuery, topicsQuery, useUpload } from '../../api/queries'
import { ErrorMessage } from '../../components/ErrorMessage'
import { JobProgress } from '../../components/JobProgress'
import { PageHeader } from '../../components/PageHeader'
import { UploadForm } from '../../components/UploadForm'

export const Route = createFileRoute('/_app/upload')({
  validateSearch: z.object({ topicId: z.string().optional() }),
  loader: ({ context }) => context.queryClient.ensureQueryData(topicsQuery),
  component: UploadPage,
})

function UploadPage() {
  const { topicId } = Route.useSearch()
  const { data: topics } = useSuspenseQuery(topicsQuery)
  const upload = useUpload()

  return (
    // Centre the form vertically in the space below the page padding (5rem) on desktop;
    // on mobile it starts at the top so the form isn't pushed below the fold.
    <div className="mx-auto flex max-w-2xl flex-col justify-center md:min-h-[calc(100svh-5rem)]">
      <PageHeader
        title="Upload a PDF"
        subtitle="We'll generate a reusable bank of multiple-choice, true/false and select-all questions."
      />
      {upload.data === undefined ? (
        <Card>
          <CardContent className="space-y-4">
            {upload.isError && <ErrorMessage error={upload.error} title="Upload failed" />}
            <UploadForm
              topics={topics}
              defaultTopicId={topicId}
              pending={upload.isPending}
              onSubmit={(input) => upload.mutate(input)}
              onInvalid={() => upload.reset()}
            />
          </CardContent>
        </Card>
      ) : (
        <UploadStatus
          jobId={upload.data.jobId}
          chapterId={upload.data.chapterId}
          onReset={() => upload.reset()}
        />
      )}
    </div>
  )
}

/** The upload's progress; once ready, what to do next, quizzing it first. */
function UploadStatus({
  jobId,
  chapterId,
  onReset,
}: {
  jobId: string
  chapterId: string
  onReset: () => void
}) {
  // Shares JobProgress's polling query, so this adds no requests.
  const { data: job } = useQuery(jobQuery(jobId))
  const another = (
    <Button variant="outline" size="lg" onClick={onReset}>
      Upload another PDF
    </Button>
  )
  return (
    <div className="space-y-4">
      <JobProgress
        jobId={jobId}
        chapterId={chapterId}
        ready={
          <div className="mt-2 flex flex-wrap gap-2">
            <Button size="lg" asChild>
              <Link to="/quizzes" search={{ chapterId }}>
                Quiz this chapter
              </Link>
            </Button>
            <Button variant="outline" size="lg" asChild>
              <Link to="/chapters/$chapterId" params={{ chapterId }}>
                Go to chapter
              </Link>
            </Button>
            {another}
          </div>
        }
      />
      {job?.status !== 'succeeded' && another}
    </div>
  )
}
