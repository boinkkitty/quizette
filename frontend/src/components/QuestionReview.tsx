import { CheckIcon, XIcon } from 'lucide-react'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { cn } from '@/lib/utils'

import type { Answer, Question } from '../api/schemas'
import { formatPoints, questionTypeLabel } from '../lib/format'

/** Shows a question with its correct answer(s) and, optionally, the user's answer. */
export function QuestionReview({ index, question, answer }: { index: number; question: Question; answer?: Answer | undefined }) {
  const correct = new Set(question.correct ?? [])
  const selected = new Set(answer?.selected ?? [])
  return (
    <Card>
      <CardContent className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span className="text-xs font-medium text-muted-foreground">
            Question {index + 1} · {questionTypeLabel[question.type]}
          </span>
          {answer !== undefined && (
            <Badge variant={answer.correct ? 'success' : answer.points > 0 ? 'warning' : 'destructive'}>
              {formatPoints(answer.points)} / 1 point
            </Badge>
          )}
        </div>
        <p className="font-medium">{question.prompt}</p>
        <ul className="space-y-1.5">
          {question.options.map((opt, i) => {
            const isCorrect = correct.has(i)
            const isSelected = selected.has(i)
            return (
              <li
                key={opt}
                className={cn(
                  'flex items-center justify-between gap-3 rounded-lg border px-3 py-2 text-sm',
                  isCorrect ? 'border-green-200 bg-green-50' : isSelected ? 'border-red-200 bg-red-50' : 'border-border',
                )}
              >
                <span className="flex items-center gap-2">
                  {isCorrect ? <CheckIcon className="size-4 text-green-600" aria-hidden /> : isSelected ? <XIcon className="size-4 text-destructive" aria-hidden /> : <span className="size-4" />}
                  {opt}
                </span>
                <span className="text-xs text-muted-foreground">
                  {isCorrect && 'Correct'}
                  {isSelected && (isCorrect ? ' · your pick' : 'Your pick')}
                </span>
              </li>
            )
          })}
        </ul>
        {question.explanation !== undefined && <p className="text-sm text-muted-foreground">{question.explanation}</p>}
        {question.sourceHint !== undefined && question.sourceHint !== '' && <p className="text-xs text-muted-foreground/80">Source: {question.sourceHint}</p>}
      </CardContent>
    </Card>
  )
}
