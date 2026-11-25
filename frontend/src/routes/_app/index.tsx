// PROTOTYPE (wayfinder #9, "What does the Dashboard show?").
// Three variants of the Dashboard on the existing `/` route, switched with ?variant=A|B|C.
// ?state=real|new|empty swaps in fixture data to judge the new-user and empty states.
// Uses existing data only (topics, chapters, submitted attempts). There is no "continue your
// quiz" item: unfinished quizzes never reach the server (ADR 0011). Links point at today's routes
// where they exist, and at the sitemap's future URLs (/quizzes?topicId=…) where they don't.
// Throwaway: lives on prototype/dashboard, never merged.
import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import {
  AlertTriangleIcon,
  ArrowRightIcon,
  CheckCircle2Icon,
  FileUpIcon,
  FolderIcon,
  LayersIcon,
  SparklesIcon,
  TargetIcon,
} from 'lucide-react'
import type { ReactNode } from 'react'
import { z } from 'zod'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { allAttemptsQuery, chaptersQuery, topicsQuery } from '../../api/queries'
import type { AttemptHistoryItem, ChapterSummary, Topic } from '../../api/schemas'
import { PageHeader } from '../../components/PageHeader'
import { PrototypeSwitcher } from '../../components/PrototypeSwitcher'
import { Spinner } from '../../components/Spinner'
import { formatDate, formatPct } from '../../lib/format'

const allChapters = chaptersQuery()

const VARIANTS = [
  { key: 'A', name: 'Next up' },
  { key: 'B', name: 'Scoreboard' },
  { key: 'C', name: 'Activity feed' },
  { key: 'D', name: 'B + feed (chosen)' },
] as const

const STATES = ['real', 'new', 'empty'] as const
type DataState = (typeof STATES)[number]

export const Route = createFileRoute('/_app/')({
  validateSearch: z.object({
    variant: z.enum(['A', 'B', 'C', 'D']).optional(),
    state: z.enum(STATES).optional(),
  }),
  loader: ({ context }) =>
    Promise.all([
      context.queryClient.ensureQueryData(topicsQuery),
      context.queryClient.ensureQueryData(allChapters),
      context.queryClient.ensureQueryData(allAttemptsQuery),
    ]),
  component: DashboardPrototype,
})

// ---------- shared model ----------

interface ChapterPerf {
  chapter: ChapterSummary
  lastPct: number | null
  lastAt: string | null
}

interface TopicPerf {
  topic: Topic
  ready: number
  pool: number
  quizzed: number
  avgLastPct: number | null
}

interface Model {
  topics: Topic[]
  chapters: ChapterSummary[]
  attempts: AttemptHistoryItem[]
  processing: ChapterSummary[]
  failed: ChapterSummary[]
  ready: ChapterSummary[]
  chapterPerf: ChapterPerf[]
  weakest: ChapterPerf[]
  untried: ChapterSummary[]
  topicPerf: TopicPerf[]
  avgPct: number | null
  last7: number
  questionsAnswered: number
}

function buildModel(
  topics: Topic[],
  chapters: ChapterSummary[],
  allAttempts: AttemptHistoryItem[],
): Model {
  const attempts = allAttempts.filter((a) => a.status === 'submitted')
  const lastByChapter = new Map<string, AttemptHistoryItem>()
  for (const a of attempts) if (!lastByChapter.has(a.chapterId)) lastByChapter.set(a.chapterId, a)

  const ready = chapters.filter((c) => c.status === 'ready')
  const chapterPerf = ready.map((c) => {
    const last = lastByChapter.get(c.id)
    return { chapter: c, lastPct: last?.scorePct ?? null, lastAt: last?.submittedAt ?? null }
  })
  const weakest = chapterPerf
    .filter((p) => p.lastPct !== null && p.lastPct < 80)
    .sort((a, b) => (a.lastPct ?? 0) - (b.lastPct ?? 0))
  const untried = ready.filter((c) => c.attemptCount === 0)

  const topicPerf = topics.map((t) => {
    const mine = chapterPerf.filter((p) => p.chapter.topicId === t.id)
    const scored = mine.filter((p) => p.lastPct !== null)
    return {
      topic: t,
      ready: mine.length,
      pool: mine.reduce((n, p) => n + p.chapter.questionCount, 0),
      quizzed: scored.length,
      avgLastPct:
        scored.length === 0
          ? null
          : scored.reduce((n, p) => n + (p.lastPct ?? 0), 0) / scored.length,
    }
  })

  const weekAgo = Date.now() - 7 * 24 * 3600_000
  return {
    topics,
    chapters,
    attempts,
    processing: chapters.filter((c) => c.status === 'pending'),
    failed: chapters.filter((c) => c.status === 'failed'),
    ready,
    chapterPerf,
    weakest,
    untried,
    topicPerf,
    avgPct:
      attempts.length === 0 ? null : attempts.reduce((n, a) => n + a.scorePct, 0) / attempts.length,
    last7: attempts.filter((a) => Date.parse(a.submittedAt ?? a.startedAt) > weekAgo).length,
    questionsAnswered: attempts.reduce((n, a) => n + a.count, 0),
  }
}

/** Fixture data for the states real data can't show on demand. */
function fixture(state: DataState): Model | null {
  if (state === 'empty') return buildModel([], [], [])
  if (state === 'new') {
    const now = new Date().toISOString()
    const topic: Topic = { id: 'fx-t', name: 'Cell biology', chapterCount: 1, createdAt: now }
    const chapter: ChapterSummary = {
      id: 'fx-c',
      topicId: 'fx-t',
      topicName: 'Cell biology',
      title: 'Mitosis and meiosis',
      status: 'pending',
      questionCount: 0,
      attemptCount: 0,
      bestScorePct: null,
      createdAt: now,
    }
    return buildModel([topic], [chapter], [])
  }
  return null
}

// ---------- route ----------

function DashboardPrototype() {
  const { variant = 'A', state = 'real' } = Route.useSearch()
  const navigate = useNavigate({ from: '/' })
  const { data: topics } = useSuspenseQuery(topicsQuery)
  const { data: chapters } = useSuspenseQuery(allChapters)
  const { data: attempts } = useSuspenseQuery(allAttemptsQuery)
  const m = fixture(state) ?? buildModel(topics, chapters, attempts)

  return (
    <>
      {variant === 'A' && <VariantA m={m} />}
      {variant === 'B' && <VariantB m={m} />}
      {variant === 'C' && <VariantC m={m} />}
      {variant === 'D' && <VariantB m={m} withFeed />}
      <PrototypeSwitcher
        variants={VARIANTS}
        current={variant}
        onChange={(k) =>
          void navigate({
            search: (s) => ({ ...s, variant: k as 'A' | 'B' | 'C' | 'D' }),
            replace: true,
          })
        }
      />
      {import.meta.env.DEV && (
        <div className="fixed top-14 left-1/2 z-50 flex -translate-x-1/2 gap-1 rounded-full bg-zinc-900 p-1 text-xs text-white shadow-lg ring-2 ring-fuchsia-500">
          {STATES.map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => void navigate({ search: (p) => ({ ...p, state: s }), replace: true })}
              className={cn('rounded-full px-2.5 py-1', s === state && 'bg-fuchsia-500')}
            >
              {s}
            </button>
          ))}
        </div>
      )}
    </>
  )
}

// ---------- shared bits ----------

const topicQuizHref = (topicId: string) => `/quizzes?topicId=${topicId}`
const chapterQuizHref = (chapterId: string) => `/quizzes?chapterId=${chapterId}`

function ScorePill({ pct }: { pct: number | null }) {
  if (pct === null) return <span className="text-xs text-muted-foreground">Not quizzed</span>
  return (
    <span
      className={cn(
        'rounded-md px-1.5 py-0.5 text-xs font-medium tabular-nums',
        pct >= 80
          ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300'
          : pct >= 50
            ? 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300'
            : 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300',
      )}
    >
      {formatPct(pct)}
    </span>
  )
}

function Section({
  title,
  action,
  children,
  className,
}: {
  title: string
  action?: ReactNode
  children: ReactNode
  className?: string | undefined
}) {
  return (
    <section className={cn('min-w-0', className)}>
      <div className="mb-3 flex items-baseline justify-between gap-2">
        <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
          {title}
        </h2>
        {action}
      </div>
      {children}
    </section>
  )
}

function NewUserHero() {
  return (
    <div className="rounded-2xl border border-dashed border-primary/40 bg-secondary/40 p-8 text-center sm:p-12">
      <FileUpIcon className="mx-auto size-10 text-primary" aria-hidden />
      <h2 className="mt-4 text-xl font-semibold">Turn a PDF into a quiz</h2>
      <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">
        Upload any PDF (notes, a manual, a paper) and Quizette writes a question bank from it. Your
        scores and progress show up here.
      </p>
      <Button size="lg" className="mt-6" asChild>
        <Link to="/upload">Upload a PDF</Link>
      </Button>
    </div>
  )
}

function ProcessingList({ m }: { m: Model }) {
  if (m.processing.length === 0 && m.failed.length === 0) return null
  return (
    <ul className="divide-y rounded-xl border bg-card">
      {m.processing.map((c) => (
        <li key={c.id}>
          <Link
            to="/chapters/$chapterId"
            params={{ chapterId: c.id }}
            className="flex items-center gap-3 px-4 py-3 hover:bg-secondary/40"
          >
            <Spinner className="size-4 text-primary" />
            <span className="min-w-0 flex-1">
              <span className="block truncate font-medium">{c.title}</span>
              <span className="block truncate text-xs text-muted-foreground">
                {c.topicName} · generating questions
              </span>
            </span>
          </Link>
        </li>
      ))}
      {m.failed.map((c) => (
        <li key={c.id}>
          <Link
            to="/chapters/$chapterId"
            params={{ chapterId: c.id }}
            className="flex items-center gap-3 px-4 py-3 hover:bg-secondary/40"
          >
            <AlertTriangleIcon className="size-4 text-destructive" aria-hidden />
            <span className="min-w-0 flex-1">
              <span className="block truncate font-medium">{c.title}</span>
              <span className="block truncate text-xs text-muted-foreground">
                {c.topicName} · generation failed
              </span>
            </span>
            <Badge variant="destructive">Failed</Badge>
          </Link>
        </li>
      ))}
    </ul>
  )
}

function RecentResults({ m, limit }: { m: Model; limit: number }) {
  if (m.attempts.length === 0)
    return (
      <p className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
        No quizzes taken yet.
      </p>
    )
  return (
    <ul className="divide-y rounded-xl border bg-card">
      {m.attempts.slice(0, limit).map((a) => (
        <li key={a.id}>
          <Link
            to="/attempts/$attemptId/results"
            params={{ attemptId: a.id }}
            className="flex items-center gap-3 px-4 py-3 hover:bg-secondary/40"
          >
            <span className="min-w-0 flex-1">
              <span className="block truncate font-medium">{a.chapterTitle}</span>
              <span className="block truncate text-xs text-muted-foreground">
                {a.topicName} · {a.count} questions · {formatDate(a.submittedAt ?? a.startedAt)}
              </span>
            </span>
            <ScorePill pct={a.scorePct} />
          </Link>
        </li>
      ))}
    </ul>
  )
}

// ---------- Variant A: Next up (action-first) ----------

type Suggestion = {
  icon: ReactNode
  eyebrow: string
  title: string
  detail: string
  href: string
  cta: string
}

function suggest(m: Model): Suggestion | null {
  const weak = m.weakest[0]
  if (weak !== undefined)
    return {
      icon: <TargetIcon className="size-5" />,
      eyebrow: 'Your weakest chapter',
      title: weak.chapter.title,
      detail: `${weak.chapter.topicName} · last score ${formatPct(weak.lastPct ?? 0)}`,
      href: chapterQuizHref(weak.chapter.id),
      cta: 'Practise this chapter',
    }
  const fresh = m.untried[0]
  if (fresh !== undefined)
    return {
      icon: <SparklesIcon className="size-5" />,
      eyebrow: 'Not quizzed yet',
      title: fresh.title,
      detail: `${fresh.topicName} · ${fresh.questionCount} questions ready`,
      href: chapterQuizHref(fresh.id),
      cta: 'Take your first quiz',
    }
  const topic = [...m.topicPerf].filter((t) => t.ready > 0).sort((a, b) => b.ready - a.ready)[0]
  if (topic !== undefined)
    return {
      icon: <LayersIcon className="size-5" />,
      eyebrow: 'Mix it up',
      title: topic.topic.name,
      detail: `${topic.ready} chapters · ${topic.pool} questions`,
      href: topicQuizHref(topic.topic.id),
      cta: 'Quiz the whole topic',
    }
  return null
}

function VariantA({ m }: { m: Model }) {
  const s = suggest(m)
  const empty = m.chapters.length === 0
  return (
    <>
      <PageHeader title="Dashboard" />
      {empty ? (
        <NewUserHero />
      ) : (
        <div className="space-y-8">
          {s === null ? (
            <div className="rounded-2xl border bg-card p-6">
              <p className="font-medium">Your first question bank is on its way.</p>
              <p className="mt-1 text-sm text-muted-foreground">
                Generating usually takes a minute or two. You can quiz it once it's ready.
              </p>
            </div>
          ) : (
            <a
              href={s.href}
              className="group block rounded-2xl border-2 border-primary/30 bg-gradient-to-br from-secondary/70 to-card p-6 transition-colors hover:border-primary sm:p-8"
            >
              <p className="flex items-center gap-2 text-sm font-medium text-primary">
                {s.icon} {s.eyebrow}
              </p>
              <p className="mt-3 text-2xl font-semibold tracking-tight">{s.title}</p>
              <p className="mt-1 text-sm text-muted-foreground">{s.detail}</p>
              <span className="mt-6 inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground">
                {s.cta}
                <ArrowRightIcon className="size-4 transition-transform group-hover:translate-x-0.5" />
              </span>
            </a>
          )}

          {m.topicPerf.some((t) => t.ready > 0) && (
            <Section title="Or quiz a whole topic">
              <div className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 sm:mx-0 sm:flex-wrap sm:px-0">
                {m.topicPerf
                  .filter((t) => t.ready > 0)
                  .map((t) => (
                    <a
                      key={t.topic.id}
                      href={topicQuizHref(t.topic.id)}
                      className="flex shrink-0 items-center gap-2 rounded-full border bg-card px-4 py-2 text-sm hover:border-primary"
                    >
                      <FolderIcon className="size-4 text-primary" aria-hidden />
                      {t.topic.name}
                      <span className="text-muted-foreground">{t.ready}</span>
                    </a>
                  ))}
              </div>
            </Section>
          )}

          <div className="grid gap-8 lg:grid-cols-2">
            <Section
              title="Recent results"
              action={
                <Link to="/history" className="text-sm font-medium text-primary hover:underline">
                  All history
                </Link>
              }
            >
              <RecentResults m={m} limit={5} />
            </Section>
            {(m.processing.length > 0 || m.failed.length > 0) && (
              <Section title="Needs attention">
                <ProcessingList m={m} />
              </Section>
            )}
          </div>
        </div>
      )}
    </>
  )
}

// ---------- Variant B: Scoreboard (numbers-first) ----------

function Stat({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="rounded-xl border bg-card p-4">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 text-2xl font-semibold tabular-nums">{value}</p>
      {hint !== undefined && <p className="mt-0.5 text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}

function ScoreStrip({ attempts }: { attempts: AttemptHistoryItem[] }) {
  const last = attempts.slice(0, 20).reverse()
  return (
    <div className="rounded-xl border bg-card p-4">
      <div className="flex h-24 items-end gap-1" role="img" aria-label="Last 20 quiz scores">
        {Array.from({ length: 20 - last.length }, (_, i) => (
          <span key={`pad-${String(i)}`} className="h-full flex-1" />
        ))}
        {last.map((a) => (
          <a
            key={a.id}
            href={`/attempts/${a.id}/results`}
            title={`${a.chapterTitle}: ${formatPct(a.scorePct)} · ${formatDate(a.submittedAt ?? a.startedAt)}`}
            className="group flex h-full flex-1 items-end justify-center"
          >
            <span
              className="w-full max-w-3 rounded-t bg-primary/70 group-hover:bg-primary"
              style={{ height: `${String(Math.max(4, a.scorePct))}%` }}
            />
          </a>
        ))}
      </div>
      <div className="mt-2 flex justify-between text-xs text-muted-foreground">
        <span>Older</span>
        <span>Latest</span>
      </div>
    </div>
  )
}

function VariantB({ m, withFeed = false }: { m: Model; withFeed?: boolean }) {
  if (m.chapters.length === 0)
    return (
      <>
        <PageHeader title="Dashboard" />
        <NewUserHero />
      </>
    )
  return (
    <>
      <PageHeader
        title="Dashboard"
        subtitle={
          m.processing.length + m.failed.length > 0 ? (
            <span className="inline-flex flex-wrap gap-2">
              {m.processing.length > 0 && (
                <Badge variant="info">
                  <Spinner className="size-3" /> {m.processing.length} generating
                </Badge>
              )}
              {m.failed.length > 0 && <Badge variant="destructive">{m.failed.length} failed</Badge>}
            </span>
          ) : undefined
        }
        actions={
          <Button size="lg" asChild>
            <Link to="/quizzes">Start a quiz</Link>
          </Button>
        }
      />
      <div className="space-y-8">
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <Stat
            label="Average score"
            value={m.avgPct === null ? '–' : formatPct(m.avgPct)}
            hint={`over ${m.attempts.length} quizzes`}
          />
          <Stat label="Quizzes this week" value={String(m.last7)} />
          <Stat label="Questions answered" value={String(m.questionsAnswered)} />
          <Stat
            label="Chapters quizzed"
            value={`${m.chapterPerf.filter((p) => p.lastPct !== null).length} / ${m.ready.length}`}
            hint="ready chapters"
          />
        </div>

        {!withFeed && m.attempts.length > 0 && (
          <Section title="Recent scores">
            <ScoreStrip attempts={m.attempts} />
          </Section>
        )}

        <Section title="By topic">
          <ul className="divide-y rounded-xl border bg-card">
            {m.topicPerf.map((t) => (
              <li
                key={t.topic.id}
                className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3"
              >
                <Link
                  to="/topics/$topicId"
                  params={{ topicId: t.topic.id }}
                  className="min-w-0 flex-1 basis-40"
                >
                  <span className="block truncate font-medium hover:text-primary">
                    {t.topic.name}
                  </span>
                  <span className="block text-xs text-muted-foreground">
                    {t.quizzed} of {t.ready} chapters quizzed
                  </span>
                </Link>
                <div className="flex w-full items-center gap-2 sm:w-48">
                  <div className="h-2 flex-1 rounded-full bg-muted">
                    <div
                      className="h-2 rounded-full bg-primary"
                      style={{ width: `${String(t.avgLastPct ?? 0)}%` }}
                    />
                  </div>
                  <span className="w-10 text-right text-sm tabular-nums">
                    {t.avgLastPct === null ? '–' : formatPct(t.avgLastPct)}
                  </span>
                </div>
                {t.ready > 0 ? (
                  <Button variant="outline" size="sm" asChild>
                    <a href={topicQuizHref(t.topic.id)}>Quiz topic</a>
                  </Button>
                ) : (
                  <span className="text-xs text-muted-foreground">Nothing ready</span>
                )}
              </li>
            ))}
          </ul>
        </Section>

        {withFeed && <ActivityFeed m={m} />}

        {!withFeed && m.weakest.length > 0 && (
          <Section title="Lowest last scores">
            <ul className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {m.weakest.slice(0, 6).map((p) => (
                <li key={p.chapter.id}>
                  <a
                    href={chapterQuizHref(p.chapter.id)}
                    className="flex items-center gap-3 rounded-xl border bg-card px-4 py-3 hover:border-primary"
                  >
                    <span className="min-w-0 flex-1">
                      <span className="block truncate font-medium">{p.chapter.title}</span>
                      <span className="block truncate text-xs text-muted-foreground">
                        {p.chapter.topicName}
                      </span>
                    </span>
                    <ScorePill pct={p.lastPct} />
                  </a>
                </li>
              ))}
            </ul>
          </Section>
        )}
      </div>
    </>
  )
}

// ---------- Variant C: Activity feed (time-first) ----------

type FeedItem =
  | { kind: 'attempt'; at: string; a: AttemptHistoryItem }
  | { kind: 'chapter'; at: string; c: ChapterSummary }

function dayLabel(iso: string): string {
  const d = new Date(iso)
  const today = new Date()
  const yest = new Date(Date.now() - 24 * 3600_000)
  if (d.toDateString() === today.toDateString()) return 'Today'
  if (d.toDateString() === yest.toDateString()) return 'Yesterday'
  return d.toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'short' })
}

function FeedRow({ item }: { item: FeedItem }) {
  if (item.kind === 'attempt') {
    const a = item.a
    return (
      <Link
        to="/attempts/$attemptId/results"
        params={{ attemptId: a.id }}
        className="flex items-start gap-3 rounded-lg px-2 py-2.5 hover:bg-secondary/50"
      >
        <CheckCircle2Icon className="mt-0.5 size-4 shrink-0 text-primary" aria-hidden />
        <span className="min-w-0 flex-1 text-sm">
          Quizzed <span className="font-medium">{a.chapterTitle}</span>
          <span className="block text-xs text-muted-foreground">
            {a.topicName} · {a.count} questions
          </span>
        </span>
        <ScorePill pct={a.scorePct} />
      </Link>
    )
  }
  const c = item.c
  return (
    <Link
      to="/chapters/$chapterId"
      params={{ chapterId: c.id }}
      className="flex items-start gap-3 rounded-lg px-2 py-2.5 hover:bg-secondary/50"
    >
      {c.status === 'pending' ? (
        <Spinner className="mt-0.5 size-4 shrink-0 text-primary" />
      ) : c.status === 'failed' ? (
        <AlertTriangleIcon className="mt-0.5 size-4 shrink-0 text-destructive" aria-hidden />
      ) : (
        <FileUpIcon className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
      )}
      <span className="min-w-0 flex-1 text-sm">
        Uploaded <span className="font-medium">{c.title}</span>
        <span className="block text-xs text-muted-foreground">
          {c.topicName} ·{' '}
          {c.status === 'pending'
            ? 'generating questions…'
            : c.status === 'failed'
              ? 'generation failed'
              : `${c.questionCount} questions`}
        </span>
      </span>
    </Link>
  )
}

function feedGroups(m: Model): { label: string; items: FeedItem[] }[] {
  const items: FeedItem[] = [
    ...m.attempts.map((a) => ({ kind: 'attempt' as const, at: a.submittedAt ?? a.startedAt, a })),
    ...m.chapters.map((c) => ({ kind: 'chapter' as const, at: c.createdAt, c })),
  ].sort((x, y) => Date.parse(y.at) - Date.parse(x.at))

  const groups: { label: string; items: FeedItem[] }[] = []
  for (const it of items.slice(0, 30)) {
    const label = dayLabel(it.at)
    const g = groups.at(-1)
    if (g?.label === label) g.items.push(it)
    else groups.push({ label, items: [it] })
  }
  return groups
}

function ActivityFeed({ m, className }: { m: Model; className?: string }) {
  return (
    <Section title="Activity" className={className}>
      <div className="space-y-6">
        {feedGroups(m).map((g) => (
          <div key={g.label}>
            <p className="mb-1 px-2 text-xs font-medium text-muted-foreground">{g.label}</p>
            <ul className="rounded-xl border bg-card p-1">
              {g.items.map((it) => (
                <li key={`${it.kind}-${it.kind === 'attempt' ? it.a.id : it.c.id}`}>
                  <FeedRow item={it} />
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </Section>
  )
}

function VariantC({ m }: { m: Model }) {
  const quickStart = m.topicPerf.filter((t) => t.ready > 0)

  return (
    <>
      <PageHeader
        title="Dashboard"
        actions={
          <Button variant="outline" asChild>
            <Link to="/upload">Upload PDF</Link>
          </Button>
        }
      />
      {m.chapters.length === 0 ? (
        <NewUserHero />
      ) : (
        <div className="grid gap-8 lg:grid-cols-[1fr_18rem]">
          <aside className="min-w-0 space-y-6 lg:order-2">
            <Section title="Quick start">
              {quickStart.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  Nothing to quiz yet. Questions are still generating.
                </p>
              ) : (
                <ul className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 lg:mx-0 lg:flex-col lg:px-0">
                  {quickStart.map((t) => (
                    <li key={t.topic.id} className="shrink-0">
                      <a
                        href={topicQuizHref(t.topic.id)}
                        className="flex min-w-48 items-center gap-3 rounded-xl border bg-card px-3 py-2.5 hover:border-primary"
                      >
                        <FolderIcon className="size-4 text-primary" aria-hidden />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-sm font-medium">{t.topic.name}</span>
                          <span className="block text-xs text-muted-foreground">
                            {t.ready} chapters · {t.pool} questions
                          </span>
                        </span>
                        <ScorePill pct={t.avgLastPct} />
                      </a>
                    </li>
                  ))}
                </ul>
              )}
            </Section>
            {m.weakest[0] !== undefined && (
              <Section title="Worth another go" className="hidden lg:block">
                <a
                  href={chapterQuizHref(m.weakest[0].chapter.id)}
                  className="block rounded-xl border bg-card px-3 py-2.5 text-sm hover:border-primary"
                >
                  <span className="block truncate font-medium">{m.weakest[0].chapter.title}</span>
                  <span className="text-xs text-muted-foreground">
                    last score {formatPct(m.weakest[0].lastPct ?? 0)}
                  </span>
                </a>
              </Section>
            )}
          </aside>
          <ActivityFeed m={m} className="lg:order-1" />
        </div>
      )}
    </>
  )
}
