import { CheckIcon, EyeIcon, EyeOffIcon } from 'lucide-react'
import { type ReactNode, useState } from 'react'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'

import type { Question } from '../api/schemas'
import { questionTypeLabel } from '../lib/format'

interface Props {
  /** Questions with answers revealed (from the question-set endpoint). */
  questions: readonly Question[]
  /** Rendered in place of Next on the last question, e.g. a link back. */
  finish?: ReactNode
}

/**
 * Study mode: the quiz layout, one question per screen, but nothing to answer.
 * Each question's answer stays hidden until "Show answer" is pressed; the choice
 * is remembered when moving back and forth.
 */
export function QuestionWalkthrough({ questions, finish }: Props) {
  const [index, setIndex] = useState(0)
  const [revealed, setRevealed] = useState<ReadonlySet<string>>(() => new Set())
  const question = questions[index]
  if (question === undefined) return null

  const shown = revealed.has(question.id)
  const isLast = index === questions.length - 1
  const correct = (question.correct ?? []).flatMap((i) => {
    const text = question.options[i]
    return text === undefined ? [] : [text]
  })
  const toggle = (): void => {
    setRevealed((prev) => {
      const next = new Set(prev)
      if (shown) next.delete(question.id)
      else next.add(question.id)
      return next
    })
  }

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <div className="flex items-center justify-between text-sm text-muted-foreground">
        <span>
          Question {index + 1} of {questions.length}
        </span>
        <span>
          {revealed.size} {revealed.size === 1 ? 'answer' : 'answers'} shown
        </span>
      </div>
      <Progress value={((index + 1) / questions.length) * 100} aria-label="Progress" className="h-1.5" />

      <Card>
        <CardContent className="space-y-4">
          <p className="text-xs font-medium tracking-wide text-primary uppercase">{questionTypeLabel[question.type]}</p>
          <h2 className="text-lg font-medium">{question.prompt}</h2>

          {shown ? (
            <div className="space-y-3 rounded-lg border border-green-200 bg-green-50 p-4" aria-live="polite">
              <div>
                <p className="text-xs font-medium text-green-800 uppercase">{correct.length > 1 ? 'Answers' : 'Answer'}</p>
                <ul className="mt-1 space-y-1">
                  {correct.map((text) => (
                    <li key={text} className="flex items-center gap-2 font-medium text-green-900">
                      <CheckIcon className="size-4 shrink-0" aria-hidden />
                      {text}
                    </li>
                  ))}
                </ul>
              </div>
              {question.explanation !== undefined && question.explanation !== '' && <p className="text-sm text-green-900">{question.explanation}</p>}
              {question.sourceHint !== undefined && question.sourceHint !== '' && (
                <p className="text-xs text-green-800">Source: {question.sourceHint}</p>
              )}
            </div>
          ) : null}

          <Button variant={shown ? 'outline' : 'secondary'} onClick={toggle}>
            {shown ? <EyeOffIcon aria-hidden /> : <EyeIcon aria-hidden />}
            {shown ? 'Hide answer' : 'Show answer'}
          </Button>
        </CardContent>
      </Card>

      <div className="flex justify-between">
        <Button variant="outline" size="lg" disabled={index === 0} onClick={() => setIndex((i) => i - 1)}>
          Previous
        </Button>
        {isLast ? (
          finish
        ) : (
          <Button size="lg" onClick={() => setIndex((i) => i + 1)}>
            Next
          </Button>
        )}
      </div>
    </div>
  )
}
