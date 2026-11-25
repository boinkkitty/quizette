import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import { Button } from '@/components/ui/button'

import { useSubmitQuiz } from '../../../api/queries'
import { ConfirmDialog } from '../../../components/ConfirmDialog'
import { ErrorMessage } from '../../../components/ErrorMessage'
import { PageHeader } from '../../../components/PageHeader'
import { QuizRunner } from '../../../components/QuizRunner'
import { clearQuiz, loadQuiz, saveQuizProgress } from '../../../lib/quizSlot'

/** Takes the tab's unfinished quiz (ADR 0011). With none, there is nothing to take. */
export const Route = createFileRoute('/_app/quizzes/run')({
  beforeLoad: () => {
    if (loadQuiz() === null) throw redirect({ to: '/quizzes', replace: true })
  },
  component: RunQuizPage,
})

function RunQuizPage() {
  // Read once: from here on the runner owns the answers and saves them back to the slot.
  const [saved] = useState(loadQuiz)
  if (saved === null) return null
  return <RunQuiz saved={saved} />
}

function RunQuiz({ saved }: { saved: NonNullable<ReturnType<typeof loadQuiz>> }) {
  const { quiz } = saved
  const submit = useSubmitQuiz(quiz)
  const navigate = useNavigate()

  return (
    <>
      <PageHeader
        title={quiz.chapterTitle}
        subtitle={`${String(quiz.questions.length)}-question quiz`}
        actions={
          <ConfirmDialog
            trigger={
              <Button variant="outline" size="lg">
                Quit
              </Button>
            }
            title="Quit this quiz?"
            description="Your answers won't be saved."
            confirmLabel="Quit quiz"
            cancelLabel="Keep answering"
            destructive
            onConfirm={() => {
              clearQuiz()
              void navigate({ to: '/quizzes', search: { chapterId: quiz.chapterId } })
            }}
          />
        }
      />
      {submit.isError && <ErrorMessage error={submit.error} title="Could not submit" />}
      <QuizRunner
        questions={quiz.questions}
        initial={saved}
        onProgress={saveQuizProgress}
        submitting={submit.isPending}
        onSubmit={(responses) => {
          submit.mutate(responses, {
            onSuccess: (attempt) => {
              clearQuiz()
              // Replace the runner, so Back from the results doesn't reopen an empty quiz.
              void navigate({
                to: '/attempts/$attemptId/results',
                params: { attemptId: attempt.id },
                replace: true,
              })
            },
          })
        }}
      />
    </>
  )
}
