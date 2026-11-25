// PROTOTYPE (wayfinder #12, "How History and Results show a topic quiz").
// Three variants of History and Results on the existing `/history` route, switched with ?variant=A|B|C.
// ?attempt=<id> shows that attempt's Results in place (standing in for the future /history/:attemptId).
// Uses fixture attempts only: topic-scoped attempts and soft-deleted content don't exist yet (ADR 0010).
// Fixtures cover a topic quiz with one deleted chapter, a quiz on a deleted topic, a quiz on a deleted
// chapter, and ordinary chapter quizzes. Throwaway: lives on prototype/history-results, never merged.
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import {
  ArrowLeftIcon,
  ArrowRightIcon,
  ChevronDownIcon,
  FileTextIcon,
  HistoryIcon,
  LayersIcon,
  RotateCcwIcon,
  Trash2Icon,
} from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { z } from 'zod'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { cn } from '@/lib/utils'

import type { Answer, Question } from '../../api/schemas'
import { PageHeader } from '../../components/PageHeader'
import { PrototypeSwitcher } from '../../components/PrototypeSwitcher'
import { QuestionReview } from '../../components/QuestionReview'
import { formatDate, formatPct, formatPoints } from '../../lib/format'

const VARIANTS = [
  { key: 'A', name: 'Table + breakdown table' },
  { key: 'B', name: 'Cards + review by chapter' },
  { key: 'C', name: 'Rows + chapter filter chips' },
] as const
type VariantKey = (typeof VARIANTS)[number]['key']

export const Route = createFileRoute('/_app/history')({
  validateSearch: z.object({
    variant: z.enum(['A', 'B', 'C']).optional(),
    attempt: z.string().optional(),
  }),
  component: HistoryPrototype,
})

// ---------- fixtures ----------

interface Ch {
  id: string
  title: string
  deleted?: boolean
}
interface Tp {
  id: string
  name: string
  deleted?: boolean
}
interface Item {
  question: Question
  chapter: Ch
  answer: Answer
}
interface Att {
  id: string
  scope: 'topic' | 'chapter'
  topic: Tp
  chapter?: Ch
  submittedAt: string
  items: Item[]
}

const PROMPTS: Record<string, string[]> = {
  cells: [
    'Which organelle produces most of the cell’s ATP?',
    'What does the nucleolus assemble?',
    'Which structure is found in plant cells but not animal cells?',
  ],
  genetics: [
    'What is the expected ratio of a monohybrid cross between two heterozygotes?',
    'Which base pairs with adenine in DNA?',
    'What is a codon?',
  ],
  ecology: [
    'What is a trophic level?',
    'Which of these is a density-dependent factor?',
    'What does carrying capacity describe?',
  ],
  membranes: [
    'Which molecules pass the lipid bilayer most easily?',
    'What drives facilitated diffusion?',
    'Select all that are features of the fluid mosaic model.',
  ],
  preterite: [
    'Conjugate “hablar” in the preterite, yo form.',
    'Which verb is irregular in the preterite?',
  ],
  subjunctive: [
    'Which trigger requires the subjunctive?',
    'Pick the subjunctive form of “tener”, ella.',
  ],
  tax: [
    'What is the filing deadline?',
    'Which receipts must be kept?',
    'What is the standard deduction?',
  ],
}

const cells: Ch = { id: 'ch-cells', title: 'Cell structure' }
const genetics: Ch = { id: 'ch-gen', title: 'Genetics and inheritance' }
const ecology: Ch = { id: 'ch-eco', title: 'Ecology' }
const membranes: Ch = { id: 'ch-mem', title: 'Cell membranes and transport', deleted: true }
const preterite: Ch = { id: 'ch-pre', title: 'Preterite tense', deleted: true }
const subjunctive: Ch = { id: 'ch-sub', title: 'Subjunctive mood', deleted: true }
const tax: Ch = { id: 'ch-tax', title: '2025 tax return guide' }

const biology: Tp = { id: 'tp-bio', name: 'Biology 101' }
const spanish: Tp = { id: 'tp-es', name: 'Spanish verbs', deleted: true }
const admin: Tp = { id: 'tp-admin', name: 'Household admin' }

let qid = 0
/** Builds n questions from a chapter; `wrong` lists 0-based indexes answered wrongly, `half` partially. */
function make(ch: Ch, key: string, n: number, wrong: number[] = [], half: number[] = []): Item[] {
  return Array.from({ length: n }, (_, i) => {
    const id = `q${String(++qid)}`
    const prompts = PROMPTS[key] ?? ['?']
    const isMrq = half.includes(i)
    const question: Question = {
      id,
      position: i,
      type: isMrq ? 'mrq' : 'mcq',
      prompt: prompts[i % prompts.length] ?? '?',
      options: ['Option one', 'Option two', 'Option three', 'Option four'],
      correct: isMrq ? [0, 2] : [1],
      explanation: 'Explanation from the source document.',
      sourceHint: `${ch.title}, p. ${String(3 + i * 2)}`,
    }
    const answer: Answer = wrong.includes(i)
      ? { questionId: id, selected: [3], points: 0, correct: false }
      : isMrq
        ? { questionId: id, selected: [0], points: 0.5, correct: false }
        : { questionId: id, selected: [1], points: 1, correct: true }
    return { question, chapter: ch, answer }
  })
}

/** Interleaves chapters the way a random draw would. */
function shuffle(items: Item[]): Item[] {
  return items
    .map((it, i) => ({ it, k: (i * 7919) % 31 }))
    .sort((a, b) => a.k - b.k)
    .map((x) => x.it)
}

const ATTEMPTS: Att[] = [
  {
    id: 't1',
    scope: 'topic',
    topic: biology,
    submittedAt: '2026-11-25T09:40:00Z',
    items: shuffle([
      ...make(cells, 'cells', 6, [2]),
      ...make(genetics, 'genetics', 6, [0, 4], [5]),
      ...make(ecology, 'ecology', 5, [1, 2, 3]),
      ...make(membranes, 'membranes', 3, [], [2]),
    ]),
  },
  {
    id: 'c1',
    scope: 'chapter',
    topic: biology,
    chapter: genetics,
    submittedAt: '2026-11-24T18:05:00Z',
    items: make(genetics, 'genetics', 10, [3, 7]),
  },
  {
    id: 'c2',
    scope: 'chapter',
    topic: biology,
    chapter: membranes,
    submittedAt: '2026-11-22T12:30:00Z',
    items: make(membranes, 'membranes', 5, [1, 4]),
  },
  {
    id: 't2',
    scope: 'topic',
    topic: spanish,
    submittedAt: '2026-11-20T20:15:00Z',
    items: shuffle([
      ...make(preterite, 'preterite', 5, [1]),
      ...make(subjunctive, 'subjunctive', 5, [0, 2, 3]),
    ]),
  },
  {
    id: 'c3',
    scope: 'chapter',
    topic: admin,
    chapter: tax,
    submittedAt: '2026-11-18T08:00:00Z',
    items: make(tax, 'tax', 5),
  },
]

// ---------- derived ----------

interface ChScore {
  chapter: Ch
  count: number
  points: number
  pct: number
}

function score(items: Item[]) {
  const points = items.reduce((s, it) => s + it.answer.points, 0)
  return { points, max: items.length, pct: items.length === 0 ? 0 : (points / items.length) * 100 }
}

function byChapter(a: Att): ChScore[] {
  const m = new Map<string, Item[]>()
  for (const it of a.items) m.set(it.chapter.id, [...(m.get(it.chapter.id) ?? []), it])
  return [...m.values()].map((its) => {
    const s = score(its)
    return { chapter: (its[0] as Item).chapter, count: its.length, points: s.points, pct: s.pct }
  })
}

/** The scope is gone when the topic, or for a chapter quiz the chapter, was deleted. */
function scopeDeleted(a: Att): boolean {
  return a.topic.deleted === true || (a.scope === 'chapter' && a.chapter?.deleted === true)
}

function chapterCount(a: Att): number {
  return new Set(a.items.map((it) => it.chapter.id)).size
}

function pctTone(pct: number): string {
  return pct >= 80 ? 'bg-green-500' : pct >= 50 ? 'bg-amber-400' : 'bg-red-400'
}

// ---------- page ----------

function HistoryPrototype() {
  const { variant = 'A', attempt } = Route.useSearch()
  const navigate = useNavigate({ from: '/history' })
  const open = (id?: string) =>
    void navigate({ search: (s) => ({ ...s, attempt: id }), resetScroll: true })
  const selected = ATTEMPTS.find((a) => a.id === attempt)

  return (
    <>
      <PrototypeSwitcher
        variants={VARIANTS}
        current={variant}
        onChange={(v) => void navigate({ search: (s) => ({ ...s, variant: v as VariantKey }) })}
      />
      {selected === undefined ? (
        <HistoryList variant={variant} onOpen={open} />
      ) : (
        <Results variant={variant} attempt={selected} onBack={() => open(undefined)} />
      )}
    </>
  )
}

/** Dashed box showing what the Dashboard feed would say, so the label is judged with the variant. */
function FeedLabelNote({ variant }: { variant: VariantKey }) {
  const label = {
    A: 'Quizzed Biology 101 (whole topic)',
    B: 'Quizzed Biology 101 · 4 chapters',
    C: 'Quizzed all of Biology 101',
  }[variant]
  return (
    <p className="mb-6 rounded-lg border border-dashed border-fuchsia-400 px-3 py-2 text-xs text-fuchsia-700">
      Prototype note: the Dashboard feed would read “{label}” for a topic quiz, and “Quizzed
      Genetics and inheritance” for a chapter quiz.
    </p>
  )
}

function DeletedBadge({ what }: { what: string }) {
  return (
    <Badge variant="outline" className="text-muted-foreground">
      <Trash2Icon aria-hidden />
      Deleted {what}
    </Badge>
  )
}

/** A chapter or topic name: a link while it exists, plain muted text once deleted. */
function ScopeName({
  name,
  deleted,
  to,
  className,
}: {
  name: string
  deleted?: boolean | undefined
  to: string
  className?: string
}) {
  if (deleted === true)
    return <span className={cn('text-muted-foreground', className)}>{name}</span>
  return (
    <a href={to} className={cn('hover:text-primary hover:underline', className)}>
      {name}
    </a>
  )
}

function HistoryList({ variant, onOpen }: { variant: VariantKey; onOpen: (id: string) => void }) {
  return (
    <>
      <PageHeader title="History" subtitle="Every quiz you’ve submitted, newest first." />
      <FeedLabelNote variant={variant} />
      {variant === 'A' && <HistoryA onOpen={onOpen} />}
      {variant === 'B' && <HistoryB onOpen={onOpen} />}
      {variant === 'C' && <HistoryC onOpen={onOpen} />}
    </>
  )
}

// ---------- A: table, stacked on mobile ----------

function QuizCellA({ a }: { a: Att }) {
  if (a.scope === 'topic') {
    return (
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-1.5">
          <ScopeName
            name={a.topic.name}
            deleted={a.topic.deleted}
            to={`/topics/${a.topic.id}`}
            className="font-medium"
          />
          <Badge variant="info">Whole topic</Badge>
          {a.topic.deleted === true && <DeletedBadge what="topic" />}
        </div>
        <p className="text-xs text-muted-foreground">{chapterCount(a)} chapters</p>
      </div>
    )
  }
  return (
    <div className="min-w-0">
      <div className="flex flex-wrap items-center gap-1.5">
        <ScopeName
          name={a.chapter?.title ?? ''}
          deleted={a.chapter?.deleted}
          to={`/chapters/${a.chapter?.id ?? ''}`}
          className="font-medium"
        />
        {a.chapter?.deleted === true && <DeletedBadge what="chapter" />}
      </div>
      <p className="text-xs text-muted-foreground">{a.topic.name}</p>
    </div>
  )
}

function HistoryA({ onOpen }: { onOpen: (id: string) => void }) {
  return (
    <>
      <div className="hidden rounded-xl border bg-card md:block">
        <table className="w-full text-left text-sm">
          <thead className="border-b bg-muted text-xs text-muted-foreground uppercase">
            <tr>
              <th className="px-5 py-2 font-medium">Date</th>
              <th className="px-5 py-2 font-medium">Quiz</th>
              <th className="px-5 py-2 font-medium">Questions</th>
              <th className="px-5 py-2 font-medium">Score</th>
              <th className="px-5 py-2">
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {ATTEMPTS.map((a) => {
              const s = score(a.items)
              return (
                <tr key={a.id}>
                  <td className="px-5 py-3 whitespace-nowrap">{formatDate(a.submittedAt)}</td>
                  <td className="px-5 py-3">
                    <QuizCellA a={a} />
                  </td>
                  <td className="px-5 py-3">{a.items.length}</td>
                  <td className="px-5 py-3 font-medium whitespace-nowrap">
                    {formatPoints(s.points)} / {s.max}{' '}
                    <span className="text-muted-foreground">({formatPct(s.pct)})</span>
                  </td>
                  <td className="px-5 py-3 text-right">
                    <button
                      type="button"
                      className="font-medium text-primary hover:underline"
                      onClick={() => onOpen(a.id)}
                    >
                      Review
                    </button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
      <ul className="divide-y rounded-xl border bg-card md:hidden">
        {ATTEMPTS.map((a) => {
          const s = score(a.items)
          return (
            <li key={a.id}>
              <button
                type="button"
                className="flex w-full items-start justify-between gap-3 px-4 py-3 text-left"
                onClick={() => onOpen(a.id)}
              >
                <div className="min-w-0">
                  <QuizCellA a={a} />
                  <p className="mt-1 text-xs text-muted-foreground">
                    {formatDate(a.submittedAt)} · {a.items.length} questions
                  </p>
                </div>
                <span className="shrink-0 font-medium">{formatPct(s.pct)}</span>
              </button>
            </li>
          )
        })}
      </ul>
    </>
  )
}

// ---------- B: cards with a per-chapter split bar ----------

function SplitBar({ a }: { a: Att }) {
  const parts = byChapter(a)
  return (
    <div className="flex h-2 w-full gap-0.5 overflow-hidden rounded-full">
      {parts.map((p) => (
        <div
          key={p.chapter.id}
          title={`${p.chapter.title}${p.chapter.deleted === true ? ' (deleted)' : ''}: ${formatPct(p.pct)}`}
          className={cn(pctTone(p.pct), p.chapter.deleted === true && 'opacity-35')}
          style={{ flexGrow: p.count }}
        />
      ))}
    </div>
  )
}

function HistoryB({ onOpen }: { onOpen: (id: string) => void }) {
  return (
    <ul className="grid gap-3 sm:grid-cols-2">
      {ATTEMPTS.map((a) => {
        const s = score(a.items)
        const parts = byChapter(a)
        const title = a.scope === 'topic' ? a.topic.name : (a.chapter?.title ?? '')
        const gone = scopeDeleted(a)
        return (
          <li key={a.id}>
            <button
              type="button"
              onClick={() => onOpen(a.id)}
              className="flex h-full w-full flex-col gap-3 rounded-xl border bg-card p-4 text-left hover:border-primary/40"
            >
              <div className="flex w-full items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    {a.scope === 'topic' ? (
                      <LayersIcon className="size-3.5" aria-hidden />
                    ) : (
                      <FileTextIcon className="size-3.5" aria-hidden />
                    )}
                    {a.scope === 'topic'
                      ? `Topic · ${String(parts.length)} chapters`
                      : `Chapter · ${a.topic.name}`}
                  </p>
                  <p className={cn('mt-0.5 font-medium', gone && 'text-muted-foreground')}>
                    {title}
                  </p>
                </div>
                <div className="shrink-0 text-right">
                  <p className="text-lg font-semibold">{formatPct(s.pct)}</p>
                  <p className="text-xs text-muted-foreground">
                    {formatPoints(s.points)} / {s.max}
                  </p>
                </div>
              </div>
              {a.scope === 'topic' ? (
                <SplitBar a={a} />
              ) : (
                <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
                  <div
                    className={cn('h-full', pctTone(s.pct), gone && 'opacity-35')}
                    style={{ width: `${String(s.pct)}%` }}
                  />
                </div>
              )}
              <div className="flex w-full items-center justify-between text-xs text-muted-foreground">
                <span>{formatDate(a.submittedAt)}</span>
                {gone && <DeletedBadge what={a.scope === 'topic' ? 'topic' : 'chapter'} />}
              </div>
            </button>
          </li>
        )
      })}
    </ul>
  )
}

// ---------- C: compact rows with a scope filter ----------

function HistoryC({ onOpen }: { onOpen: (id: string) => void }) {
  const [filter, setFilter] = useState<'all' | 'topic' | 'chapter'>('all')
  const shown = ATTEMPTS.filter((a) => filter === 'all' || a.scope === filter)
  return (
    <>
      <div className="mb-4 flex gap-1.5">
        {(
          [
            ['all', 'All'],
            ['topic', 'Topic quizzes'],
            ['chapter', 'Chapter quizzes'],
          ] as const
        ).map(([k, label]) => (
          <button
            key={k}
            type="button"
            onClick={() => setFilter(k)}
            className={cn(
              'min-h-9 rounded-full border px-3 text-sm',
              filter === k ? 'border-primary bg-primary text-primary-foreground' : 'bg-card',
            )}
          >
            {label}
          </button>
        ))}
      </div>
      <ul className="divide-y rounded-xl border bg-card">
        {shown.map((a) => {
          const s = score(a.items)
          const gone = scopeDeleted(a)
          return (
            <li key={a.id}>
              <button
                type="button"
                onClick={() => onOpen(a.id)}
                className="flex min-h-14 w-full items-center gap-3 px-4 py-3 text-left hover:bg-muted/50"
              >
                <span
                  className={cn(
                    'grid size-9 shrink-0 place-items-center rounded-lg',
                    a.scope === 'topic' ? 'bg-secondary text-primary' : 'bg-muted',
                    gone && 'opacity-50',
                  )}
                >
                  {a.scope === 'topic' ? (
                    <LayersIcon className="size-4" aria-hidden />
                  ) : (
                    <FileTextIcon className="size-4" aria-hidden />
                  )}
                </span>
                <div className="min-w-0 flex-1">
                  <p className={cn('truncate font-medium', gone && 'text-muted-foreground')}>
                    {a.scope === 'topic' ? `All of ${a.topic.name}` : (a.chapter?.title ?? '')}
                    {gone && <span className="font-normal"> (deleted)</span>}
                  </p>
                  <p className="truncate text-xs text-muted-foreground">
                    {a.scope === 'topic' ? `${String(chapterCount(a))} chapters` : a.topic.name} ·{' '}
                    {a.items.length} questions · {formatDate(a.submittedAt)}
                  </p>
                </div>
                <span
                  className={cn(
                    'shrink-0 rounded-full px-2.5 py-1 text-sm font-semibold',
                    s.pct >= 80
                      ? 'bg-green-50 text-green-700'
                      : s.pct >= 50
                        ? 'bg-amber-50 text-amber-800'
                        : 'bg-red-50 text-red-700',
                  )}
                >
                  {formatPct(s.pct)}
                </span>
              </button>
            </li>
          )
        })}
      </ul>
    </>
  )
}

// ---------- Results ----------

function Results({
  variant,
  attempt: a,
  onBack,
}: {
  variant: VariantKey
  attempt: Att
  onBack: () => void
}) {
  const s = score(a.items)
  const correct = a.items.filter((it) => it.answer.correct).length
  const title = a.scope === 'topic' ? a.topic.name : (a.chapter?.title ?? '')
  return (
    <div className="mx-auto max-w-2xl pb-24">
      <button
        type="button"
        onClick={onBack}
        className="mb-2 inline-flex min-h-9 items-center gap-1 text-sm text-muted-foreground hover:text-primary"
      >
        <ArrowLeftIcon className="size-4" aria-hidden /> History
      </button>
      <PageHeader
        title={title}
        subtitle={
          <span className="flex flex-wrap items-center gap-1.5">
            {a.scope === 'topic'
              ? `Whole-topic quiz · ${String(chapterCount(a))} chapters`
              : `Chapter quiz · ${a.topic.name}`}{' '}
            · {formatDate(a.submittedAt)}
            {scopeDeleted(a) && (
              <DeletedBadge what={a.topic.deleted === true ? 'topic' : 'chapter'} />
            )}
          </span>
        }
      />
      <Card className="mb-6 border-primary/20 bg-secondary/40">
        <CardContent className="flex items-center justify-between">
          <div>
            <p className="text-sm text-muted-foreground">Score</p>
            <p className="text-3xl font-semibold">
              {formatPoints(s.points)} / {s.max}
            </p>
            <p className="text-sm text-muted-foreground">
              {correct} fully correct · select-all questions earn partial credit
            </p>
          </div>
          <p className="text-4xl font-semibold text-primary">{formatPct(s.pct)}</p>
        </CardContent>
      </Card>
      {variant === 'A' && <ResultsA a={a} />}
      {variant === 'B' && <ResultsB a={a} />}
      {variant === 'C' && <ResultsC a={a} />}
      <ResultsBar a={a} onHistory={onBack} />
    </div>
  )
}

/** The sitemap's bar: Retake · Go to <scope> · History, with the first two hidden once the scope is gone. */
function ResultsBar({ a, onHistory }: { a: Att; onHistory: () => void }) {
  const gone = scopeDeleted(a)
  const name = a.scope === 'topic' ? a.topic.name : (a.chapter?.title ?? '')
  return (
    <div className="fixed inset-x-0 bottom-0 z-40 border-t bg-background/95 backdrop-blur md:left-60">
      <div className="mx-auto flex max-w-2xl items-center gap-2 px-4 py-3">
        {gone ? (
          <p className="flex-1 text-xs text-muted-foreground">
            This {a.topic.deleted === true ? 'topic' : 'chapter'} was deleted, so it can’t be
            retaken.
          </p>
        ) : (
          <>
            <Button className="min-h-11 flex-1 sm:flex-none">
              <RotateCcwIcon aria-hidden /> Retake
            </Button>
            <Button variant="outline" className="min-h-11 min-w-0 flex-1 sm:flex-none">
              <span className="truncate">Go to {name}</span>
            </Button>
          </>
        )}
        <Button variant="ghost" className="min-h-11" onClick={onHistory}>
          <HistoryIcon aria-hidden /> History
        </Button>
      </div>
    </div>
  )
}

/** Where a question came from, written above its card. */
function SourceLine({ ch, showLink = true }: { ch: Ch; showLink?: boolean }) {
  if (ch.deleted === true) {
    return (
      <p className="mb-1.5 flex items-center gap-1.5 text-xs text-muted-foreground">
        <Trash2Icon className="size-3.5" aria-hidden />
        From a deleted chapter: {ch.title}
      </p>
    )
  }
  return (
    <p className="mb-1.5 text-xs text-muted-foreground">
      From{' '}
      {showLink ? (
        <a href={`/chapters/${ch.id}`} className="font-medium hover:text-primary hover:underline">
          {ch.title}
        </a>
      ) : (
        ch.title
      )}
    </p>
  )
}

function SectionTitle({ children }: { children: ReactNode }) {
  return <h2 className="mb-3 text-sm font-semibold text-muted-foreground uppercase">{children}</h2>
}

// A: breakdown table, then a flat review with a source line on each question.
function ResultsA({ a }: { a: Att }) {
  const parts = byChapter(a)
  return (
    <>
      {a.scope === 'topic' && (
        <section className="mb-8">
          <SectionTitle>By chapter</SectionTitle>
          <ul className="divide-y rounded-xl border bg-card">
            {parts.map((p) => (
              <li key={p.chapter.id} className="flex items-center gap-3 px-4 py-3 text-sm">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <ScopeName
                      name={p.chapter.title}
                      deleted={p.chapter.deleted}
                      to={`/chapters/${p.chapter.id}`}
                      className="font-medium"
                    />
                    {p.chapter.deleted === true && <DeletedBadge what="chapter" />}
                  </div>
                  <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-muted">
                    <div
                      className={cn('h-full', pctTone(p.pct))}
                      style={{ width: `${String(p.pct)}%` }}
                    />
                  </div>
                </div>
                <span className="w-20 shrink-0 text-right whitespace-nowrap">
                  {formatPoints(p.points)} / {p.count}
                </span>
                <span className="w-10 shrink-0 text-right font-medium">{formatPct(p.pct)}</span>
              </li>
            ))}
          </ul>
        </section>
      )}
      <SectionTitle>Answers</SectionTitle>
      <ol className="space-y-4">
        {a.items.map((it, i) => (
          <li key={it.question.id}>
            {(a.scope === 'topic' || it.chapter.deleted === true) && <SourceLine ch={it.chapter} />}
            <QuestionReview index={i} question={it.question} answer={it.answer} />
          </li>
        ))}
      </ol>
    </>
  )
}

// B: the review itself is grouped into one collapsible section per chapter.
function ResultsB({ a }: { a: Att }) {
  const parts = byChapter(a)
  const [open, setOpen] = useState<Set<string>>(
    () => new Set(parts.filter((p) => p.pct < 100).map((p) => p.chapter.id)),
  )
  const toggle = (id: string) =>
    setOpen((o) => {
      const n = new Set(o)
      if (n.has(id)) n.delete(id)
      else n.add(id)
      return n
    })
  const starts = parts.map((_, i) => parts.slice(0, i).reduce((n, q) => n + q.count, 0))
  return (
    <div className="space-y-3">
      {parts.map((p, pi) => {
        const items = a.items.filter((it) => it.chapter.id === p.chapter.id)
        const start = starts[pi] ?? 0
        const isOpen = open.has(p.chapter.id)
        return (
          <section key={p.chapter.id} className="rounded-xl border bg-card">
            <div className="flex items-center gap-3 px-4 py-3">
              <button
                type="button"
                onClick={() => toggle(p.chapter.id)}
                className="flex min-h-11 min-w-0 flex-1 items-center gap-2 text-left"
                aria-expanded={isOpen}
              >
                <ChevronDownIcon
                  className={cn('size-4 shrink-0 transition-transform', !isOpen && '-rotate-90')}
                  aria-hidden
                />
                <div className="min-w-0">
                  <p
                    className={cn(
                      'truncate font-medium',
                      p.chapter.deleted === true && 'text-muted-foreground',
                    )}
                  >
                    {p.chapter.title}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {formatPoints(p.points)} / {p.count} · {formatPct(p.pct)}
                    {p.chapter.deleted === true && ' · chapter deleted'}
                  </p>
                </div>
              </button>
              {p.chapter.deleted !== true && (
                <a
                  href={`/chapters/${p.chapter.id}`}
                  className="grid size-11 shrink-0 place-items-center rounded-lg text-muted-foreground hover:bg-muted hover:text-primary"
                  aria-label={`Open ${p.chapter.title}`}
                >
                  <ArrowRightIcon className="size-4" aria-hidden />
                </a>
              )}
            </div>
            {isOpen && (
              <ol className="space-y-4 border-t bg-muted/30 p-3 sm:p-4">
                {items.map((it, i) => (
                  <li key={it.question.id}>
                    <QuestionReview index={start + i} question={it.question} answer={it.answer} />
                  </li>
                ))}
              </ol>
            )}
          </section>
        )
      })}
    </div>
  )
}

// C: chapter chips that filter a flat review, plus "only mistakes".
function ResultsC({ a }: { a: Att }) {
  const parts = byChapter(a)
  const [chapter, setChapter] = useState<string | null>(null)
  const [mistakes, setMistakes] = useState(false)
  const shown = a.items
    .map((it, i) => ({ it, i }))
    .filter(({ it }) => chapter === null || it.chapter.id === chapter)
    .filter(({ it }) => !mistakes || !it.answer.correct)
  return (
    <>
      {a.scope === 'topic' && (
        <div className="-mx-4 mb-3 flex gap-2 overflow-x-auto px-4 pb-1 md:mx-0 md:flex-wrap md:px-0">
          <Chip active={chapter === null} onClick={() => setChapter(null)}>
            All chapters
          </Chip>
          {parts.map((p) => (
            <Chip
              key={p.chapter.id}
              active={chapter === p.chapter.id}
              deleted={p.chapter.deleted}
              onClick={() => setChapter(p.chapter.id)}
            >
              {p.chapter.deleted === true && <Trash2Icon className="size-3.5" aria-hidden />}
              <span className="max-w-40 truncate">{p.chapter.title}</span>
              <span className="font-semibold">{formatPct(p.pct)}</span>
            </Chip>
          ))}
        </div>
      )}
      <label className="mb-4 flex min-h-9 items-center gap-2 text-sm">
        <input type="checkbox" checked={mistakes} onChange={(e) => setMistakes(e.target.checked)} />
        Only show mistakes
      </label>
      {chapter !== null && (
        <SourceBanner ch={(parts.find((p) => p.chapter.id === chapter) as ChScore).chapter} />
      )}
      <ol className="space-y-4">
        {shown.map(({ it, i }) => (
          <li key={it.question.id}>
            {chapter === null && (a.scope === 'topic' || it.chapter.deleted === true) && (
              <SourceLine ch={it.chapter} />
            )}
            <QuestionReview index={i} question={it.question} answer={it.answer} />
          </li>
        ))}
        {shown.length === 0 && (
          <li className="text-sm text-muted-foreground">No mistakes in this chapter.</li>
        )}
      </ol>
    </>
  )
}

function SourceBanner({ ch }: { ch: Ch }) {
  return (
    <div className="mb-4 flex items-center justify-between gap-3 rounded-lg bg-muted px-3 py-2 text-sm">
      {ch.deleted === true ? (
        <span className="text-muted-foreground">
          {ch.title} was deleted. Its questions stay here for this result.
        </span>
      ) : (
        <>
          <span>{ch.title}</span>
          <a href={`/chapters/${ch.id}`} className="font-medium text-primary hover:underline">
            Open chapter →
          </a>
        </>
      )}
    </div>
  )
}

function Chip({
  active,
  deleted,
  onClick,
  children,
}: {
  active: boolean
  deleted?: boolean | undefined
  onClick: () => void
  children: ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'flex min-h-9 shrink-0 items-center gap-1.5 rounded-full border px-3 text-sm whitespace-nowrap',
        active ? 'border-primary bg-primary text-primary-foreground' : 'bg-card',
        deleted === true && !active && 'border-dashed text-muted-foreground',
      )}
    >
      {children}
    </button>
  )
}
