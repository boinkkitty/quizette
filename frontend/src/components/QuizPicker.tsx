import { Link, useNavigate } from '@tanstack/react-router'
import {
  BookOpenIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  FolderIcon,
  LayersIcon,
} from 'lucide-react'
import { useState } from 'react'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { useDrawQuiz } from '../api/queries'
import type { ChapterSummary, ScopeRef, Topic } from '../api/schemas'
import { formatPct } from '../lib/format'
import { splitAcrossChapters } from '../lib/split'
import { startQuiz } from '../lib/quizSlot'
import { EmptyState } from './EmptyState'
import { ErrorMessage } from './ErrorMessage'
import { Spinner } from './Spinner'

/** Past performance of a topic or chapter: its scope's attempts only (ADR 0010). */
interface Perf {
  attempts: number
  lastPct: number | null
  bestPct: number | null
}

interface ChapterRow {
  id: string
  title: string
  questionCount: number
  perf: Perf
}

interface TopicRow {
  id: string
  name: string
  /** Ready chapters in creation order, the order the server splits a topic quiz in. */
  ready: ChapterRow[]
  /** Chapters still generating or failed. */
  notReady: number
  pool: number
  perf: Perf
}

/** What step 3 quizzes: a whole topic or one of its chapters. */
interface Choice {
  scope: ScopeRef
  label: string
  topic: TopicRow
  /** The chapters it draws from. */
  chapters: ChapterRow[]
  pool: number
  perf: Perf
}

function buildModel(topics: readonly Topic[], chapters: readonly ChapterSummary[]): TopicRow[] {
  return topics.map((t) => {
    const mine = chapters.filter((c) => c.topicId === t.id)
    const ready = mine
      .filter((c) => c.status === 'ready')
      // The server's order: created_at, then id (backend ListChaptersByTopic).
      .sort((a, b) => a.createdAt.localeCompare(b.createdAt) || a.id.localeCompare(b.id))
      .map((c) => ({
        id: c.id,
        title: c.title,
        questionCount: c.questionCount,
        perf: { attempts: c.attemptCount, lastPct: null, bestPct: c.bestScorePct },
      }))
    return {
      id: t.id,
      name: t.name,
      ready,
      notReady: mine.length - ready.length,
      pool: ready.reduce((s, c) => s + c.questionCount, 0),
      perf: { attempts: t.attemptCount, lastPct: t.lastScorePct, bestPct: t.bestScorePct },
    }
  })
}

function topicChoice(topic: TopicRow): Choice {
  return {
    scope: { topicId: topic.id },
    label: `All of ${topic.name}`,
    topic,
    chapters: topic.ready,
    pool: topic.pool,
    perf: topic.perf,
  }
}

function chapterChoice(topic: TopicRow, c: ChapterRow): Choice {
  return {
    scope: { chapterId: c.id },
    label: c.title,
    topic,
    chapters: [c],
    pool: c.questionCount,
    perf: c.perf,
  }
}

/** A scope from the URL, which may name anything. */
export interface Preselect {
  chapterId?: string | undefined
  topicId?: string | undefined
}

/**
 * Finds a preselected scope among the ready ones. chapterId wins if both are set. Unknown,
 * deleted or unready ids give null.
 */
function resolve(model: readonly TopicRow[], pre: Preselect): Choice | null {
  if (pre.chapterId !== undefined) {
    for (const t of model) {
      const c = t.ready.find((x) => x.id === pre.chapterId)
      if (c !== undefined) return chapterChoice(t, c)
    }
    return null
  }
  if (pre.topicId !== undefined) {
    const t = model.find((x) => x.id === pre.topicId)
    return t === undefined || t.ready.length === 0 ? null : topicChoice(t)
  }
  return null
}

const plural = (n: number, one: string, many = `${one}s`) => `${String(n)} ${n === 1 ? one : many}`

function PerfLine({ perf, className }: { perf: Perf; className?: string }) {
  if (perf.attempts === 0) return <span className={className}>Not quizzed yet</span>
  const parts = [plural(perf.attempts, 'quiz', 'quizzes')]
  if (perf.lastPct !== null) parts.push(`last ${formatPct(perf.lastPct)}`)
  if (perf.bestPct !== null) parts.push(`best ${formatPct(perf.bestPct)}`)
  return <span className={className}>{parts.join(' · ')}</span>
}

export interface QuizPickerProps {
  topics: readonly Topic[]
  /** Every chapter of the user's, in any status. */
  chapters: readonly ChapterSummary[]
  /**
   * A scope to open step 3 with; chapterId wins if both are set. It is read on mount only,
   * so give the picker a key that changes with it.
   */
  preselect?: Preselect
}

/**
 * Sets up a quiz in three steps: topic, then the whole topic or one chapter, then how many.
 * The step is component state, not the URL, so the browser's Back leaves the page.
 */
export function QuizPicker({ topics, chapters, preselect = {} }: QuizPickerProps) {
  const model = buildModel(topics, chapters)
  const [pre] = useState(() => resolve(model, preselect))
  // Back from a preselected step 3 goes to step 2 for its topic.
  const [topicId, setTopicId] = useState<string | null>(pre?.topic.id ?? null)
  const [choice, setChoice] = useState<Choice | null>(pre)
  const topic = model.find((t) => t.id === topicId) ?? null

  if (!model.some((t) => t.ready.length > 0)) {
    return (
      <EmptyState title="Nothing to quiz yet">
        <Link to="/upload" className="font-medium text-primary hover:underline">
          Upload a PDF
        </Link>{' '}
        to make your first chapter. You can quiz it once its questions are ready.
      </EmptyState>
    )
  }

  const back = () => {
    if (choice !== null) setChoice(null)
    else setTopicId(null)
  }

  const step = choice !== null ? 3 : topic !== null ? 2 : 1

  return (
    <div className="max-w-3xl">
      <StepIndicator step={step} />
      {topic !== null && (
        <Button variant="ghost" className="mb-4 -ml-2 h-11" onClick={back}>
          <ChevronLeftIcon aria-hidden /> Back
        </Button>
      )}

      {topic === null && (
        <ul className="grid gap-3 sm:grid-cols-2">
          {model.map((t) => (
            <li key={t.id}>
              <button
                type="button"
                disabled={t.ready.length === 0}
                onClick={() => setTopicId(t.id)}
                className="flex h-full w-full items-start gap-3 rounded-xl border bg-card p-5 text-left transition-colors enabled:hover:border-primary enabled:hover:bg-secondary/40 disabled:border-dashed disabled:bg-transparent"
              >
                <span className="rounded-lg bg-secondary p-2 text-primary">
                  <FolderIcon className="size-5" aria-hidden />
                </span>
                <span className="min-w-0">
                  <span className="block font-medium break-words">{t.name}</span>
                  {t.ready.length === 0 ? (
                    <span className="mt-1 block text-sm text-muted-foreground">
                      No chapters ready yet
                    </span>
                  ) : (
                    <>
                      <span className="mt-1 block text-sm text-muted-foreground">
                        {plural(t.ready.length, 'chapter')} · {plural(t.pool, 'question')}
                      </span>
                      <PerfLine perf={t.perf} className="block text-xs text-muted-foreground" />
                    </>
                  )}
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}

      {topic !== null && choice === null && (
        <div className="space-y-4">
          <button
            type="button"
            onClick={() => setChoice(topicChoice(topic))}
            className="flex w-full items-center gap-4 rounded-xl border-2 border-primary/40 bg-primary/5 p-5 text-left hover:border-primary"
          >
            <LayersIcon className="size-8 shrink-0 text-primary" aria-hidden />
            <span className="min-w-0">
              <span className="block text-lg font-semibold">Whole topic: {topic.name}</span>
              <span className="block text-sm text-muted-foreground">
                Mix of all {plural(topic.ready.length, 'ready chapter')} ·{' '}
                {plural(topic.pool, 'question')}
              </span>
              <PerfLine perf={topic.perf} className="block text-xs text-muted-foreground" />
            </span>
          </button>
          <p className="pt-2 text-sm font-medium text-muted-foreground">Or one chapter</p>
          <ul className="grid gap-3 sm:grid-cols-2">
            {topic.ready.map((c) => (
              <li key={c.id}>
                <button
                  type="button"
                  onClick={() => setChoice(chapterChoice(topic, c))}
                  className="flex h-full w-full items-start gap-3 rounded-xl border bg-card p-4 text-left hover:border-primary hover:bg-secondary/40"
                >
                  <BookOpenIcon className="mt-0.5 size-5 shrink-0 text-primary" aria-hidden />
                  <span className="min-w-0">
                    <span className="block font-medium break-words">{c.title}</span>
                    <span className="block text-sm text-muted-foreground">
                      {plural(c.questionCount, 'question')}
                    </span>
                    <PerfLine perf={c.perf} className="block text-xs text-muted-foreground" />
                  </span>
                </button>
              </li>
            ))}
          </ul>
          {topic.notReady > 0 && (
            <p className="text-sm text-muted-foreground">
              {topic.notReady} more {topic.notReady === 1 ? 'chapter is' : 'chapters are'} not ready
              yet.
            </p>
          )}
        </div>
      )}

      {choice !== null && <HowMany choice={choice} />}
    </div>
  )
}

/** The count rule (ADR 0010): a multiple of 5, at least 5 and at most min(pool, 50). */
const MAX_COUNT = 50

const STEPS = ['Topic', 'What to quiz', 'How many'] as const

/** Numbered steps; on mobile only the current step's label shows. */
function StepIndicator({ step }: { step: number }) {
  return (
    <ol aria-label="Steps" className="mb-6 flex flex-wrap items-center gap-2 text-sm">
      {STEPS.map((label, i) => {
        const n = i + 1
        return (
          <li key={label} className="flex items-center gap-2">
            {i > 0 && <ChevronRightIcon className="size-4 text-muted-foreground" aria-hidden />}
            <span
              aria-current={n === step ? 'step' : undefined}
              className={cn(
                'flex items-center gap-1.5',
                n === step ? 'font-semibold text-foreground' : 'text-muted-foreground',
              )}
            >
              <span
                className={cn(
                  'flex size-6 items-center justify-center rounded-full text-xs',
                  n <= step ? 'bg-primary text-primary-foreground' : 'bg-muted',
                )}
                aria-hidden
              >
                {n}
              </span>
              <span className={cn(n !== step && 'sr-only sm:not-sr-only')}>{label}</span>
            </span>
          </li>
        )
      })}
    </ol>
  )
}

function HowMany({ choice }: { choice: Choice }) {
  const navigate = useNavigate()
  const draw = useDrawQuiz()
  const max = Math.max(5, Math.floor(Math.min(choice.pool, MAX_COUNT) / 5) * 5)
  const [count, setCount] = useState(Math.min(10, max))

  return (
    <div className="space-y-6 rounded-xl border bg-card p-5 sm:p-6">
      <div>
        <p className="text-lg font-semibold break-words">{choice.label}</p>
        <PerfLine perf={choice.perf} className="text-sm text-muted-foreground" />
      </div>
      <div>
        <div className="flex items-baseline justify-between">
          <span className="text-sm font-medium">How many questions?</span>
          <span className="text-3xl font-semibold tabular-nums">{count}</span>
        </div>
        <input
          type="range"
          min={5}
          max={max}
          step={5}
          value={count}
          onChange={(e) => setCount(Number(e.target.value))}
          className="mt-3 h-11 w-full accent-(--primary)"
          aria-label="Number of questions"
        />
      </div>
      {choice.chapters.length > 1 && <SplitPreview chapters={choice.chapters} count={count} />}
      {draw.isError && <ErrorMessage error={draw.error} title="Could not start quiz" />}
      <Button
        size="lg"
        className="h-11 w-full"
        disabled={draw.isPending}
        onClick={() => {
          draw.mutate(
            { scope: choice.scope, count },
            {
              onSuccess: (quiz) => {
                startQuiz(quiz)
                void navigate({ to: '/quizzes/run' })
              },
            },
          )
        }}
      >
        {draw.isPending && <Spinner />} Start {count}-question quiz
      </Button>
    </div>
  )
}

/** How many questions each chapter gives, as the server will split them. */
function SplitPreview({ chapters, count }: { chapters: readonly ChapterRow[]; count: number }) {
  const shares = splitAcrossChapters(
    count,
    chapters.map((c) => c.questionCount),
  )
  return (
    <div className="space-y-2">
      <p id="split-heading" className="text-sm font-medium">
        Where they come from
      </p>
      <ul aria-labelledby="split-heading" className="space-y-2">
        {chapters.map((c, i) => {
          const k = shares[i] ?? 0
          return (
            <li key={c.id} className="flex items-center gap-3 text-sm">
              <span className="w-28 min-w-0 truncate sm:w-56">{c.title}</span>
              <span
                className="h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-muted"
                aria-hidden
              >
                <span
                  className="block h-full rounded-full bg-primary"
                  style={{ width: `${String((k / count) * 100)}%` }}
                />
              </span>
              <span className="w-6 shrink-0 text-right tabular-nums">{k}</span>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
