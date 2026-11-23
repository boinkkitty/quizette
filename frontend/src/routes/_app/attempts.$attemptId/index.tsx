import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'

import { attemptQuery, useSubmitAttempt } from '../../../api/queries'
import { ErrorMessage } from '../../../components/ErrorMessage'
import { QuizRunner } from '../../../components/QuizRunner'
import { PageHeader } from '../../../components/PageHeader'
import { clearQuizProgress } from '../../../lib/quizProgress'

export const Route = createFileRoute('/_app/attempts/$attemptId/')({
  loader: async ({ context, params }) => {
    const attempt = await context.queryClient.ensureQueryData(attemptQuery(params.attemptId))
    if (attempt.status === 'submitted') {
      clearQuizProgress(params.attemptId)
      throw redirect({ to: '/attempts/$attemptId/results', params })
    }
  },
  component: TakeQuizPage,
})

function TakeQuizPage() {
  const { attemptId } = Route.useParams()
  const { data: attempt } = useSuspenseQuery(attemptQuery(attemptId))
  const submit = useSubmitAttempt(attemptId)
  const navigate = useNavigate()

  return (
    <>
      <PageHeader
        title={attempt.chapterTitle}
        subtitle={`${String(attempt.count)}-question quiz`}
      />
      {submit.isError && <ErrorMessage error={submit.error} title="Could not submit" />}
      <QuizRunner
        key={attemptId}
        attemptId={attemptId}
        questions={attempt.questions}
        submitting={submit.isPending}
        onSubmit={(responses) => {
          submit.mutate(responses, {
            onSuccess: () => {
              clearQuizProgress(attemptId)
              void navigate({ to: '/attempts/$attemptId/results', params: { attemptId } })
            },
          })
        }}
      />
    </>
  )
}
