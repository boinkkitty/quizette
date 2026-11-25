// PROTOTYPE (wayfinder #10, "Quizzes page: choosing a topic or a chapter, and the question count").
// Three variants of the /quizzes picker on the existing route, switched with ?variant=A|B|C.
// Real topics, chapters and history; Start is a stub because the draw endpoint (ADR 0011) doesn't
// exist yet. Topic "past performance" is derived from chapter attempts, since topic attempts
// don't exist yet either. Throwaway: lives on prototype/quizzes-picker, never merged.
import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import {
  BookOpenIcon,
  CheckIcon,
  ChevronDownIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  FolderIcon,
  LayersIcon,
  MinusIcon,
  PlusIcon,
} from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { cn } from '@/lib/utils'

import { allAttemptsQuery, chaptersQuery, topicsQuery } from '../../../api/queries'
import type { AttemptHistoryItem, ChapterSummary, Topic } from '../../../api/schemas'
import { EmptyState } from '../../../components/EmptyState'
import { PageHeader } from '../../../components/PageHeader'
import { PrototypeSwitcher } from '../../../components/PrototypeSwitcher'
import { formatPct } from '../../../lib/format'

const allChapters = chaptersQuery()

export const Route = createFileRoute('/_app/quizzes/')({
  validateSearch: z.object({
    topicId: z.string().optional(),
    chapterId: z.string().optional(),
    variant: z.enum(['A', 'B', 'C']).optional(),
  }),
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.ensureQueryData(topicsQuery),
      context.queryClient.ensureQueryData(allChapters),
      context.queryClient.ensureQueryData(allAttemptsQuery),
    ]),
  component: QuizzesPrototype,
})

// ---------- shared model ----------

interface Perf {
  attempts: number
  bestPct: number | null
  lastPct: number | null
  lastAt: string | null
}

interface ChapterRow {
  id: string
  title: string
  topicId: string
  questionCount: number
  perf: Perf
}

interface TopicRow {
  id: string
  name: string
  ready: ChapterRow[]
  notReady: number
  pool: number
  perf: Perf
}

type Scope = { kind: 'topic'; topicId: string } | { kind: 'chapter'; chapterId: string }

const MAX_COUNT = 50

function perfOf(attempts: readonly AttemptHistoryItem[]): Perf {
  const done = attempts
    .filter((a) => a.status === 'submitted' && a.submittedAt !== null)
    .sort((a, b) => (b.submittedAt ?? '').localeCompare(a.submittedAt ?? ''))
  return {
    attempts: done.length,
    bestPct: done.length === 0 ? null : Math.max(...done.map((a) => a.scorePct)),
    lastPct: done[0]?.scorePct ?? null,
    lastAt: done[0]?.submittedAt ?? null,
  }
}

function buildModel(
  topics: readonly Topic[],
  chapters: readonly ChapterSummary[],
  attempts: readonly AttemptHistoryItem[],
): TopicRow[] {
  return [...topics]
    .sort((a, b) => a.name.localeCompare(b.name))
    .map((t) => {
      const mine = chapters.filter((c) => c.topicId === t.id)
      const ready = mine
        .filter((c) => c.status === 'ready')
        .map((c) => ({
          id: c.id,
          title: c.title,
          topicId: t.id,
          questionCount: c.questionCount,
          perf: perfOf(attempts.filter((a) => a.chapterId === c.id)),
        }))
      return {
        id: t.id,
        name: t.name,
        ready,
        notReady: mine.length - ready.length,
        pool: ready.reduce((s, c) => s + c.questionCount, 0),
        perf: perfOf(attempts.filter((a) => a.topicId === t.id)),
      }
    })
}

/** Multiples of 5 from 5 to min(pool, 50) (ADR 0010). */
function counts(pool: number): number[] {
  const out: number[] = []
  for (let n = 5; n <= Math.min(pool, MAX_COUNT); n += 5) out.push(n)
  return out
}

function clampCount(n: number, pool: number): number {
  const max = Math.min(pool, MAX_COUNT) - (Math.min(pool, MAX_COUNT) % 5)
  return Math.max(5, Math.min(max, n))
}

/** Largest-remainder split of n across chapters by question count (ADR 0010). */
function split(chapters: readonly ChapterRow[], n: number): Map<string, number> {
  const pool = chapters.reduce((s, c) => s + c.questionCount, 0)
  const raw = chapters.map((c) => ({ id: c.id, exact: (c.questionCount * n) / pool }))
  const out = new Map(raw.map((r) => [r.id, Math.floor(r.exact)]))
  let left = n - [...out.values()].reduce((s, v) => s + v, 0)
  for (const r of [...raw].sort((a, b) => (b.exact % 1) - (a.exact % 1))) {
    if (left-- <= 0) break
    out.set(r.id, (out.get(r.id) ?? 0) + 1)
  }
  return out
}

interface Resolved {
  label: string
  topic: TopicRow
  chapters: ChapterRow[]
  pool: number
  perf: Perf
}

function resolve(model: readonly TopicRow[], scope: Scope | null): Resolved | null {
  if (scope === null) return null
  if (scope.kind === 'topic') {
    const t = model.find((x) => x.id === scope.topicId)
    if (t === undefined || t.ready.length === 0) return null
    return { label: `All of ${t.name}`, topic: t, chapters: t.ready, pool: t.pool, perf: t.perf }
  }
  for (const t of model) {
    const c = t.ready.find((x) => x.id === scope.chapterId)
    if (c !== undefined)
      return { label: c.title, topic: t, chapters: [c], pool: c.questionCount, perf: c.perf }
  }
  return null
}

function useModel() {
  const { data: topics } = useSuspenseQuery(topicsQuery)
  const { data: chapters } = useSuspenseQuery(allChapters)
  const { data: attempts } = useSuspenseQuery(allAttemptsQuery)
  return buildModel(topics, chapters, attempts)
}

function initialScope(model: readonly TopicRow[], topicId?: string, chapterId?: string) {
  const s: Scope | null =
    chapterId !== undefined
      ? { kind: 'chapter', chapterId }
      : topicId !== undefined
        ? { kind: 'topic', topicId }
        : null
  return resolve(model, s) === null ? null : s
}

function stubStart(r: Resolved, n: number) {
  const parts =
    r.chapters.length > 1
      ? ` (${[...split(r.chapters, n)].map(([id, k]) => `${r.chapters.find((c) => c.id === id)?.title}: ${k}`).join(', ')})`
      : ''
  toast.info(`Prototype: would draw ${n} questions from ${r.label}${parts} → /quizzes/run`)
}

function PerfLine({ perf, className }: { perf: Perf; className?: string }) {
  if (perf.attempts === 0) return <span className={className}>Not quizzed yet</span>
  return (
    <span className={className}>
      {perf.attempts} {perf.attempts === 1 ? 'quiz' : 'quizzes'} · last{' '}
      {formatPct(perf.lastPct ?? 0)} · best {formatPct(perf.bestPct ?? 0)}
    </span>
  )
}

function NothingReady() {
  return (
    <EmptyState title="Nothing to quiz yet">
      <Link to="/upload" className="font-medium text-primary hover:underline">
        Upload a PDF
      </Link>{' '}
      to make your first chapter. You can quiz it once its questions are ready.
    </EmptyState>
  )
}

// ---------- page ----------

const VARIANTS = [
  { key: 'A', name: 'One form' },
  { key: 'B', name: 'Library list' },
  { key: 'C', name: 'Steps' },
] as const

function QuizzesPrototype() {
  const search = Route.useSearch()
  const navigate = useNavigate({ from: Route.fullPath })
  const model = useModel()
  const variant = search.variant ?? 'A'
  const anyReady = model.some((t) => t.ready.length > 0)
  const pre = initialScope(model, search.topicId, search.chapterId)
  // Remount on variant or preselection change so each variant starts from the URL.
  const key = `${variant}:${search.topicId ?? ''}:${search.chapterId ?? ''}`

  return (
    <>
      <PageHeader title="Quizzes" subtitle="Quiz yourself on a whole topic or one chapter." />
      {!anyReady ? (
        <NothingReady />
      ) : variant === 'A' ? (
        <VariantA key={key} model={model} pre={pre} />
      ) : variant === 'B' ? (
        <VariantB key={key} model={model} pre={pre} />
      ) : (
        <VariantC key={key} model={model} pre={pre} />
      )}
      <PrototypeSwitcher
        variants={VARIANTS}
        current={variant}
        onChange={(v) =>
          void navigate({
            search: (s) => ({ ...s, variant: v as 'A' | 'B' | 'C' }),
            replace: true,
          })
        }
      />
    </>
  )
}

interface VariantProps {
  model: TopicRow[]
  pre: Scope | null
}

// ---------- A: one form ----------

function scopeValue(s: Scope | null) {
  if (s === null) return ''
  return s.kind === 'topic' ? `topic:${s.topicId}` : `chapter:${s.chapterId}`
}

function parseScope(v: string): Scope | null {
  const [kind, id = ''] = v.split(':')
  if (kind === 'topic') return { kind: 'topic', topicId: id }
  if (kind === 'chapter') return { kind: 'chapter', chapterId: id }
  return null
}

function VariantA({ model, pre }: VariantProps) {
  const [scope, setScope] = useState<Scope | null>(pre)
  const [count, setCount] = useState(10)
  const r = resolve(model, scope)
  const n = r === null ? count : clampCount(count, r.pool)

  return (
    <div className="mx-auto max-w-xl pb-24 md:pb-0">
      <div className="space-y-6 rounded-xl border bg-card p-5 sm:p-6">
        <label className="block space-y-2">
          <span className="text-sm font-medium">Quiz on</span>
          <NativeSelect
            className="w-full"
            value={scopeValue(scope)}
            onChange={(e) => setScope(parseScope(e.target.value))}
          >
            <NativeSelectOption value="">Choose a topic or chapter…</NativeSelectOption>
            {model.map((t) =>
              t.ready.length === 0 ? (
                <optgroup key={t.id} label={`${t.name}: no chapters ready`} />
              ) : (
                <optgroup key={t.id} label={t.name}>
                  <NativeSelectOption value={`topic:${t.id}`}>
                    All of {t.name} ({t.ready.length} chapters, {t.pool} questions)
                  </NativeSelectOption>
                  {t.ready.map((c) => (
                    <NativeSelectOption key={c.id} value={`chapter:${c.id}`}>
                      {c.title} ({c.questionCount})
                    </NativeSelectOption>
                  ))}
                </optgroup>
              ),
            )}
          </NativeSelect>
        </label>

        <div className="space-y-2">
          <span className="text-sm font-medium">Questions</span>
          <div className="flex flex-wrap gap-2">
            {(r === null ? counts(MAX_COUNT) : counts(r.pool)).map((k) => (
              <button
                key={k}
                type="button"
                disabled={r === null}
                onClick={() => setCount(k)}
                className={cn(
                  'min-w-12 rounded-lg border px-3 py-2 text-sm font-medium transition-colors disabled:opacity-40',
                  k === n && r !== null
                    ? 'border-primary bg-primary text-primary-foreground'
                    : 'hover:border-primary hover:bg-secondary/40',
                )}
              >
                {k}
              </button>
            ))}
          </div>
          {r !== null && r.pool < MAX_COUNT && (
            <p className="text-xs text-muted-foreground">
              {r.pool} questions available, so up to {counts(r.pool).at(-1)}.
            </p>
          )}
        </div>

        {r !== null && (
          <div className="rounded-lg bg-secondary/50 p-4 text-sm">
            <p className="font-medium">{r.label}</p>
            <PerfLine perf={r.perf} className="mt-1 block text-muted-foreground" />
            {r.chapters.length > 1 && (
              <p className="mt-2 text-xs text-muted-foreground">
                Drawn from {r.chapters.length} chapters, in proportion to their size.
                {r.topic.notReady > 0 && ` ${r.topic.notReady} not ready yet are left out.`}
              </p>
            )}
          </div>
        )}

        <div className="fixed inset-x-0 bottom-0 z-10 border-t bg-background p-4 md:static md:border-0 md:p-0">
          <Button
            size="lg"
            className="h-11 w-full"
            disabled={r === null}
            onClick={() => r !== null && stubStart(r, n)}
          >
            {r === null ? 'Choose what to quiz' : `Start ${n}-question quiz`}
          </Button>
        </div>
      </div>
    </div>
  )
}

// ---------- B: library list with a bottom action bar ----------

function VariantB({ model, pre }: VariantProps) {
  const [scope, setScope] = useState<Scope | null>(pre)
  const [count, setCount] = useState(10)
  const preTopic = resolve(model, pre)?.topic.id
  const [open, setOpen] = useState<Set<string>>(
    () => new Set(preTopic !== undefined ? [preTopic] : model.slice(0, 1).map((t) => t.id)),
  )
  const r = resolve(model, scope)
  const n = r === null ? count : clampCount(count, r.pool)
  const max = r === null ? 5 : clampCount(MAX_COUNT, r.pool)
  const isSel = (s: Scope) => scopeValue(s) === scopeValue(scope)
  const toggle = (id: string) =>
    setOpen((o) => {
      const x = new Set(o)
      if (x.has(id)) x.delete(id)
      else x.add(id)
      return x
    })

  return (
    <div className="pb-28">
      <ul className="space-y-3">
        {model.map((t) => {
          const expanded = open.has(t.id)
          const topicScope: Scope = { kind: 'topic', topicId: t.id }
          if (t.ready.length === 0)
            return (
              <li
                key={t.id}
                className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-dashed px-4 py-3 text-sm text-muted-foreground"
              >
                <span className="flex items-center gap-2">
                  <FolderIcon className="size-4" aria-hidden /> {t.name}
                </span>
                <span>
                  {t.notReady > 0 ? `${t.notReady} generating or failed · ` : 'No chapters · '}
                  <Link
                    to="/upload"
                    search={{ topicId: t.id }}
                    className="font-medium text-primary hover:underline"
                  >
                    Upload a PDF
                  </Link>
                </span>
              </li>
            )
          return (
            <li key={t.id} className="overflow-hidden rounded-xl border bg-card">
              <div
                className={cn(
                  'flex items-center gap-2 pr-2',
                  isSel(topicScope) && 'bg-primary/10 ring-2 ring-primary ring-inset',
                )}
              >
                <button
                  type="button"
                  onClick={() => setScope(topicScope)}
                  className="flex min-h-14 flex-1 items-center gap-3 px-4 py-3 text-left"
                >
                  <SelectDot on={isSel(topicScope)} />
                  <span className="min-w-0">
                    <span className="block font-medium">
                      {t.name}{' '}
                      <span className="font-normal text-muted-foreground">· whole topic</span>
                    </span>
                    <span className="block text-xs text-muted-foreground">
                      {t.ready.length} chapters · {t.pool} questions · <PerfLine perf={t.perf} />
                    </span>
                  </span>
                </button>
                <button
                  type="button"
                  onClick={() => toggle(t.id)}
                  className="rounded-lg p-3 text-muted-foreground hover:bg-muted"
                  aria-label={
                    expanded ? `Hide chapters of ${t.name}` : `Show chapters of ${t.name}`
                  }
                >
                  <ChevronDownIcon
                    className={cn('size-4 transition-transform', expanded && 'rotate-180')}
                  />
                </button>
              </div>
              {expanded && (
                <ul className="divide-y border-t">
                  {t.ready.map((c) => {
                    const cs: Scope = { kind: 'chapter', chapterId: c.id }
                    return (
                      <li key={c.id}>
                        <button
                          type="button"
                          onClick={() => setScope(cs)}
                          className={cn(
                            'flex min-h-12 w-full items-center gap-3 py-2.5 pr-4 pl-10 text-left hover:bg-muted/50',
                            isSel(cs) && 'bg-primary/10 ring-2 ring-primary ring-inset',
                          )}
                        >
                          <SelectDot on={isSel(cs)} />
                          <span className="min-w-0 flex-1">
                            <span className="block truncate text-sm font-medium">{c.title}</span>
                            <span className="block text-xs text-muted-foreground">
                              {c.questionCount} questions · <PerfLine perf={c.perf} />
                            </span>
                          </span>
                        </button>
                      </li>
                    )
                  })}
                  {t.notReady > 0 && (
                    <li className="py-2 pr-4 pl-10 text-xs text-muted-foreground">
                      {t.notReady} more not ready yet
                    </li>
                  )}
                </ul>
              )}
            </li>
          )
        })}
      </ul>

      <div className="fixed inset-x-0 bottom-0 z-40 border-t bg-background/95 backdrop-blur md:left-60">
        <div className="mx-auto flex max-w-4xl flex-wrap items-center gap-3 p-3 sm:flex-nowrap sm:p-4">
          <div className="min-w-0 flex-1 basis-full sm:basis-auto">
            <p className="truncate text-sm font-medium">{r?.label ?? 'Pick a topic or chapter'}</p>
            {r !== null && <p className="text-xs text-muted-foreground">of {r.pool} questions</p>}
          </div>
          <div className="flex items-center rounded-lg border">
            <Button
              variant="ghost"
              size="icon"
              className="size-11"
              disabled={r === null || n <= 5}
              onClick={() => setCount(n - 5)}
              aria-label="5 fewer questions"
            >
              <MinusIcon />
            </Button>
            <span className="w-10 text-center font-semibold tabular-nums">{n}</span>
            <Button
              variant="ghost"
              size="icon"
              className="size-11"
              disabled={r === null || n >= max}
              onClick={() => setCount(n + 5)}
              aria-label="5 more questions"
            >
              <PlusIcon />
            </Button>
          </div>
          <Button
            size="lg"
            className="h-11 flex-1 sm:flex-none sm:px-6"
            disabled={r === null}
            onClick={() => r !== null && stubStart(r, n)}
          >
            Start quiz
          </Button>
        </div>
      </div>
    </div>
  )
}

function SelectDot({ on }: { on: boolean }) {
  return (
    <span
      className={cn(
        'flex size-5 shrink-0 items-center justify-center rounded-full border-2',
        on ? 'border-primary bg-primary text-primary-foreground' : 'border-muted-foreground/40',
      )}
      aria-hidden
    >
      {on && <CheckIcon className="size-3" strokeWidth={3} />}
    </span>
  )
}

// ---------- C: steps ----------

function VariantC({ model, pre }: VariantProps) {
  const preR = resolve(model, pre)
  const [topicId, setTopicId] = useState<string | null>(preR?.topic.id ?? null)
  const [scope, setScope] = useState<Scope | null>(pre)
  const [count, setCount] = useState(10)
  const step = scope !== null ? 3 : topicId !== null ? 2 : 1
  const topic = model.find((t) => t.id === topicId) ?? null
  const r = resolve(model, scope)
  const n = r === null ? count : clampCount(count, r.pool)

  const back = () => {
    if (step === 3) setScope(null)
    else setTopicId(null)
  }

  return (
    <div className="mx-auto max-w-3xl">
      <ol className="mb-6 flex items-center gap-2 text-sm">
        {['Topic', 'What to quiz', 'How many'].map((label, i) => (
          <li key={label} className="flex items-center gap-2">
            {i > 0 && <ChevronRightIcon className="size-4 text-muted-foreground" />}
            <span
              className={cn(
                'flex items-center gap-1.5',
                i + 1 === step ? 'font-semibold text-foreground' : 'text-muted-foreground',
              )}
            >
              <span
                className={cn(
                  'flex size-6 items-center justify-center rounded-full text-xs',
                  i + 1 <= step ? 'bg-primary text-primary-foreground' : 'bg-muted',
                )}
              >
                {i + 1}
              </span>
              <span className={cn(i + 1 !== step && 'hidden sm:inline')}>{label}</span>
            </span>
          </li>
        ))}
      </ol>

      {step > 1 && (
        <Button variant="ghost" className="mb-4 -ml-2" onClick={back}>
          <ChevronLeftIcon /> Back
        </Button>
      )}

      {step === 1 && (
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
                  <span className="block font-medium">{t.name}</span>
                  {t.ready.length === 0 ? (
                    <span className="mt-1 block text-sm text-muted-foreground">
                      No chapters ready yet
                    </span>
                  ) : (
                    <>
                      <span className="mt-1 block text-sm text-muted-foreground">
                        {t.ready.length} chapters · {t.pool} questions
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

      {step === 2 && topic !== null && (
        <div className="space-y-4">
          <button
            type="button"
            onClick={() => setScope({ kind: 'topic', topicId: topic.id })}
            className="flex w-full items-center gap-4 rounded-xl border-2 border-primary/40 bg-primary/5 p-5 text-left hover:border-primary"
          >
            <LayersIcon className="size-8 text-primary" aria-hidden />
            <span>
              <span className="block text-lg font-semibold">Whole topic: {topic.name}</span>
              <span className="block text-sm text-muted-foreground">
                Mix of all {topic.ready.length} ready chapters · {topic.pool} questions
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
                  onClick={() => setScope({ kind: 'chapter', chapterId: c.id })}
                  className="flex h-full w-full items-start gap-3 rounded-xl border bg-card p-4 text-left hover:border-primary hover:bg-secondary/40"
                >
                  <BookOpenIcon className="mt-0.5 size-5 text-primary" aria-hidden />
                  <span className="min-w-0">
                    <span className="block font-medium">{c.title}</span>
                    <span className="block text-sm text-muted-foreground">
                      {c.questionCount} questions
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

      {step === 3 && r !== null && (
        <div className="space-y-6 rounded-xl border bg-card p-5 sm:p-6">
          <div>
            <p className="text-lg font-semibold">{r.label}</p>
            <PerfLine perf={r.perf} className="text-sm text-muted-foreground" />
          </div>
          <div>
            <div className="flex items-baseline justify-between">
              <span className="text-sm font-medium">How many questions?</span>
              <span className="text-3xl font-semibold tabular-nums">{n}</span>
            </div>
            <input
              type="range"
              min={5}
              max={clampCount(MAX_COUNT, r.pool)}
              step={5}
              value={n}
              onChange={(e) => setCount(Number(e.target.value))}
              className="mt-3 h-8 w-full accent-(--primary)"
              aria-label="Number of questions"
            />
            <div className="flex justify-between text-xs text-muted-foreground">
              <span>5</span>
              <span>{clampCount(MAX_COUNT, r.pool)}</span>
            </div>
          </div>
          {r.chapters.length > 1 && (
            <div className="space-y-2">
              <p className="text-sm font-medium">Where they come from</p>
              {[...split(r.chapters, n)].map(([id, k]) => {
                const c = r.chapters.find((x) => x.id === id)
                return (
                  <div key={id} className="flex items-center gap-3 text-sm">
                    <span className="w-40 truncate sm:w-56">{c?.title}</span>
                    <span className="h-2 flex-1 overflow-hidden rounded-full bg-muted">
                      <span
                        className="block h-full rounded-full bg-primary"
                        style={{ width: `${(k / n) * 100}%` }}
                      />
                    </span>
                    <span className="w-6 text-right tabular-nums">{k}</span>
                  </div>
                )
              })}
            </div>
          )}
          <Button size="lg" className="h-11 w-full" onClick={() => stubStart(r, n)}>
            Start {n}-question quiz
          </Button>
        </div>
      )}
    </div>
  )
}
