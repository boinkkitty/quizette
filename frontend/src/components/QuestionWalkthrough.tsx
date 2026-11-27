import { CheckIcon, EyeIcon, EyeOffIcon } from 'lucide-react'
import { type ReactNode, useState } from 'react'

import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { cn } from '@/lib/utils'

import type { Question } from '../api/schemas'
import { questionTypeLabel } from '../lib/format'

interface Props {
  /** Questions with answers revealed (from the question-set endpoint). */
  questions: readonly Question[]
  /** Rendered in place of Next on the last question, e.g. a link back. */
  finish?: ReactNode
}

/**
 * Study mode: the quiz layout, one question per screen, with its options shown but not
 * selectable. "Show answer" marks the correct option(s) in place and adds the
 * explanation; the choice is remembered when moving back and forth.
 */
export function QuestionWalkthrough({ questions, finish }: Props) {
  const [index, setIndex] = useState(0)
  const [revealed, setRevealed] = useState<ReadonlySet<string>>(() => new Set())
  const question = questions[index]
  if (question === undefined) return null

  const shown = revealed.has(question.id)
  const isLast = index === questions.length - 1
  const correct = new Set(question.correct ?? [])
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
      <Progress
        value={((index + 1) / questions.length) * 100}
        aria-label="Progress"
        className="h-1.5"
      />

      <Card>
        <CardContent className="space-y-4">
          <p className="text-xs font-medium tracking-wide text-primary uppercase">
            {questionTypeLabel[question.type]}
          </p>
          <h2 className="text-lg font-medium">{question.prompt}</h2>

          <ol className="grid gap-2" aria-label="Options">
            {question.options.map((opt, i) => {
              const isCorrect = shown && correct.has(i)
              return (
                <li
                  key={opt}
                  className={cn(
                    'flex items-center gap-3 rounded-lg border px-4 py-3 text-sm transition-colors',
                    isCorrect && 'border-green-300 bg-green-50 font-medium text-green-900',
                    shown && !isCorrect && 'text-muted-foreground',
                  )}
                >
                  <span
                    className={cn(
                      'flex size-6 shrink-0 items-center justify-center rounded-full border text-xs font-medium',
                      isCorrect
                        ? 'border-green-600 bg-green-600 text-white'
                        : 'text-muted-foreground',
                    )}
                    aria-hidden
                  >
                    {isCorrect ? <CheckIcon className="size-3.5" /> : String.fromCharCode(65 + i)}
                  </span>
                  <span className="flex-1">{opt}</span>
                  {isCorrect && <span className="text-xs font-medium text-green-700">Correct</span>}
                </li>
              )
            })}
          </ol>

          {shown &&
            ((question.explanation !== undefined && question.explanation !== '') ||
              (question.sourceHint !== undefined && question.sourceHint !== '')) && (
              <div className="space-y-2 rounded-lg bg-muted/60 p-4 text-sm" aria-live="polite">
                {question.explanation !== undefined && question.explanation !== '' && (
                  <p>{question.explanation}</p>
                )}
                {question.sourceHint !== undefined && question.sourceHint !== '' && (
                  <p className="text-xs text-muted-foreground">Source: {question.sourceHint}</p>
                )}
              </div>
            )}

          <Button variant={shown ? 'outline' : 'secondary'} size="lg" onClick={toggle}>
            {shown ? <EyeOffIcon aria-hidden /> : <EyeIcon aria-hidden />}
            {shown ? 'Hide answer' : 'Show answer'}
          </Button>
        </CardContent>
      </Card>

      <div className="flex justify-between">
        <Button
          variant="outline"
          size="lg"
          disabled={index === 0}
          onClick={() => setIndex((i) => i - 1)}
        >
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
