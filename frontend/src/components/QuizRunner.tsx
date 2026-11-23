import { useEffect, useId, useState } from 'react'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { cn } from '@/lib/utils'

import type { Question } from '../api/schemas'
import { questionTypeLabel } from '../lib/format'
import { loadQuizProgress, saveQuizProgress } from '../lib/quizProgress'
import { Spinner } from './Spinner'

export type Responses = ReadonlyMap<string, readonly number[]>

interface Props {
  /** Progress is saved under this attempt and restored after a reload. */
  attemptId: string
  questions: readonly Question[]
  submitting: boolean
  onSubmit: (responses: Responses) => void
}

const optionClass = (checked: boolean) =>
  cn(
    'flex cursor-pointer items-center gap-3 rounded-lg border px-4 py-3 text-sm font-normal transition-colors',
    checked ? 'border-primary bg-secondary text-secondary-foreground' : 'border-border hover:bg-muted',
  )

/** One question per screen. MCQ / True-False use radios; MRQ uses checkboxes. */
export function QuizRunner({ attemptId, questions, submitting, onSubmit }: Props) {
  const idPrefix = useId()
  const [saved] = useState(() => loadQuizProgress(attemptId, questions.map((q) => q.id)))
  const [index, setIndex] = useState(saved.index)
  const [responses, setResponses] = useState<Map<string, number[]>>(saved.responses)
  // Keep the saved copy in step with every answer and page change.
  useEffect(() => {
    saveQuizProgress(attemptId, { index, responses })
  }, [attemptId, index, responses])
  const [confirming, setConfirming] = useState(false)
  const question = questions[index]
  if (question === undefined) return null

  const selected = responses.get(question.id) ?? []
  const multi = question.type === 'mrq'
  const answered = questions.filter((q) => (responses.get(q.id)?.length ?? 0) > 0).length
  const unanswered = questions.length - answered
  const isLast = index === questions.length - 1
  const optionId = (i: number) => `${idPrefix}-${question.id}-${String(i)}`

  const setAnswer = (next: number[]) => {
    setResponses((prev) => new Map(prev).set(question.id, next))
  }
  const toggle = (option: number) => {
    setAnswer(selected.includes(option) ? selected.filter((o) => o !== option) : [...selected, option].sort((a, b) => a - b))
  }

  const submit = () => {
    if (unanswered > 0) {
      setConfirming(true)
      return
    }
    onSubmit(responses)
  }

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>
          Question {index + 1} of {questions.length}
        </span>
        <span>{answered} answered</span>
      </div>
      <Progress value={((index + 1) / questions.length) * 100} aria-label="Quiz progress" className="h-1.5" />

      <Card>
        <CardContent className="space-y-4">
          <p className="text-xs font-medium tracking-wide text-primary uppercase">{questionTypeLabel[question.type]}</p>
          <fieldset>
            <legend className="text-lg font-medium">{question.prompt}</legend>
            {multi && <p className="mt-1 text-sm text-muted-foreground">Select all that apply. Wrong picks cancel out right ones.</p>}
            {multi ? (
              <div className="mt-4 grid gap-2">
                {question.options.map((opt, i) => (
                  <Label key={opt} htmlFor={optionId(i)} className={optionClass(selected.includes(i))}>
                    <Checkbox id={optionId(i)} checked={selected.includes(i)} onCheckedChange={() => toggle(i)} />
                    {opt}
                  </Label>
                ))}
              </div>
            ) : (
              <RadioGroup
                key={question.id}
                className="mt-4"
                value={selected[0] === undefined ? '' : String(selected[0])}
                onValueChange={(v) => setAnswer([Number(v)])}
              >
                {question.options.map((opt, i) => (
                  <Label key={opt} htmlFor={optionId(i)} className={optionClass(selected.includes(i))}>
                    <RadioGroupItem id={optionId(i)} value={String(i)} />
                    {opt}
                  </Label>
                ))}
              </RadioGroup>
            )}
          </fieldset>
        </CardContent>
      </Card>

      <div className="flex justify-between">
        <Button variant="outline" size="lg" disabled={index === 0} onClick={() => setIndex((i) => i - 1)}>
          Previous
        </Button>
        {isLast ? (
          <Button size="lg" onClick={submit} disabled={submitting}>
            {submitting && <Spinner />} Submit quiz
          </Button>
        ) : (
          <Button size="lg" onClick={() => setIndex((i) => i + 1)}>
            Next
          </Button>
        )}
      </div>

      <AlertDialog open={confirming} onOpenChange={setConfirming}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Submit with unanswered questions?</AlertDialogTitle>
            <AlertDialogDescription>
              You have {unanswered} unanswered question{unanswered === 1 ? '' : 's'}. They will score 0.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Keep answering</AlertDialogCancel>
            <AlertDialogAction onClick={() => onSubmit(responses)}>Submit anyway</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
