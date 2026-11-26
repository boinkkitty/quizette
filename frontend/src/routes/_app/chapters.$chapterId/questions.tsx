import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'

import { Button } from '@/components/ui/button'

import { chapterQuery, questionSetQuery } from '../../../api/queries'
import { EmptyState } from '../../../components/EmptyState'
import { PageHeader } from '../../../components/PageHeader'
import { QuestionWalkthrough } from '../../../components/QuestionWalkthrough'

export const Route = createFileRoute('/_app/chapters/$chapterId/questions')({
  loader: async ({ context, params }) => {
    await Promise.all([
      context.queryClient.ensureQueryData(chapterQuery(params.chapterId)),
      context.queryClient.ensureQueryData(questionSetQuery(params.chapterId)),
    ])
  },
  component: ViewQuestionsPage,
})

/** Walk through a chapter's whole question bank, revealing answers on demand. */
function ViewQuestionsPage() {
  const { chapterId } = Route.useParams()
  const { data: chapter } = useSuspenseQuery(chapterQuery(chapterId))
  const { data: set } = useSuspenseQuery(questionSetQuery(chapterId))
  const back = (
    <Link
      to="/chapters/$chapterId"
      params={{ chapterId }}
      className="hover:text-primary hover:underline"
    >
      ← {chapter.title}
    </Link>
  )
  return (
    <>
      <PageHeader title={chapter.title} subtitle={back} />
      {set.questions.length === 0 ? (
        <EmptyState title="This chapter has no questions" />
      ) : (
        <QuestionWalkthrough
          questions={set.questions}
          finish={
            <Button size="lg" asChild>
              <Link to="/chapters/$chapterId" params={{ chapterId }}>
                Done
              </Link>
            </Button>
          }
        />
      )}
    </>
  )
}
