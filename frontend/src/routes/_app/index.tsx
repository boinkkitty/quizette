import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { AlertTriangleIcon, CheckCircle2Icon, FileUpIcon, Loader2Icon } from 'lucide-react'
import type { ReactNode } from 'react'

import { dashboardQuery } from '../../api/queries'
import type { ActivityItem, Dashboard } from '../../api/schemas'
import { PageHeader } from '../../components/PageHeader'
import { ScorePill } from '../../components/ScorePill'
import { Badge } from '../../components/ui/badge'
import { Button } from '../../components/ui/button'
import { formatPct } from '../../lib/format'

export const Route = createFileRoute('/_app/')({
  loader: ({ context }) => context.queryClient.ensureQueryData(dashboardQuery),
  component: DashboardPage,
})

function DashboardPage() {
  const { data: d } = useSuspenseQuery(dashboardQuery)
  if (d.chapterCount === 0)
    return (
      <>
        <PageHeader title="Dashboard" />
        <Hero />
      </>
    )
  return (
    <>
      <PageHeader
        title="Dashboard"
        subtitle={<InProgress generating={d.generatingCount} failed={d.failedCount} />}
        actions={
          <Button size="lg" asChild>
            <Link to="/quizzes">Start a quiz</Link>
          </Button>
        }
      />
      <div className="space-y-8">
        <Stats stats={d.stats} />
        <ByTopic topics={d.topics} />
        <Activity items={d.activity} />
      </div>
    </>
  )
}

function Hero() {
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

function InProgress({ generating, failed }: { generating: number; failed: number }) {
  if (generating === 0 && failed === 0) return null
  return (
    <span className="inline-flex flex-wrap gap-2">
      {generating > 0 && (
        <Badge variant="info">
          <Loader2Icon className="size-3 animate-spin" aria-hidden /> {generating} generating
        </Badge>
      )}
      {failed > 0 && <Badge variant="destructive">{failed} failed</Badge>}
    </span>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  const id = `dashboard-${title.toLowerCase().replaceAll(' ', '-')}`
  return (
    <section aria-labelledby={id} className="min-w-0">
      <h2
        id={id}
        className="mb-3 text-sm font-semibold tracking-wide text-muted-foreground uppercase"
      >
        {title}
      </h2>
      {children}
    </section>
  )
}

function Stat({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="min-w-0 rounded-xl border bg-card p-4">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 text-2xl font-semibold tabular-nums">{value}</p>
      {hint !== undefined && <p className="mt-0.5 text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}

function Stats({ stats: s }: { stats: Dashboard['stats'] }) {
  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <Stat
        label="Average score"
        value={s.averageScorePct === null ? '–' : formatPct(s.averageScorePct)}
        hint={`over ${String(s.attemptCount)} ${s.attemptCount === 1 ? 'quiz' : 'quizzes'}`}
      />
      <Stat label="Quizzes this week" value={String(s.quizzesThisWeek)} />
      <Stat label="Questions answered" value={String(s.questionsAnswered)} />
      <Stat
        label="Chapters quizzed"
        value={`${String(s.chaptersQuizzed)} / ${String(s.readyChapters)}`}
        hint="ready chapters"
      />
    </div>
  )
}

function ByTopic({ topics }: { topics: Dashboard['topics'] }) {
  if (topics.length === 0) return null
  return (
    <Section title="By topic">
      <ul aria-label="By topic" className="divide-y rounded-xl border bg-card">
        {topics.map((t) => (
          <li key={t.id} className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3">
            <Link
              to="/topics/$topicId"
              params={{ topicId: t.id }}
              className="min-w-0 flex-1 basis-40"
            >
              <span className="block truncate font-medium hover:text-primary">{t.name}</span>
              <span className="block text-xs text-muted-foreground">
                {t.chaptersQuizzed} of {t.readyChapters} chapters quizzed
              </span>
            </Link>
            <div className="flex w-full items-center gap-2 sm:w-48">
              <div className="h-2 flex-1 rounded-full bg-muted">
                <div
                  className="h-2 rounded-full bg-primary"
                  style={{ width: `${String(t.averageLastScorePct ?? 0)}%` }}
                />
              </div>
              <span className="w-10 text-right text-sm tabular-nums">
                {t.averageLastScorePct === null ? '–' : formatPct(t.averageLastScorePct)}
              </span>
            </div>
            {t.readyChapters > 0 ? (
              <Button variant="outline" size="sm" asChild>
                <Link to="/quizzes" search={{ topicId: t.id }}>
                  Quiz topic
                </Link>
              </Button>
            ) : (
              <span className="text-xs text-muted-foreground">Nothing ready</span>
            )}
          </li>
        ))}
      </ul>
    </Section>
  )
}

/** Names the local day of iso relative to now: Today, Yesterday, or its weekday and date. */
export function dayLabel(iso: string, now: Date = new Date()): string {
  const d = new Date(iso)
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1)
  if (d.toDateString() === now.toDateString()) return 'Today'
  if (d.toDateString() === yesterday.toDateString()) return 'Yesterday'
  return d.toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'short' })
}

/** Splits items, newest first, into runs that fall on the same local day. */
function byDay(items: ActivityItem[]): { label: string; items: ActivityItem[] }[] {
  const now = new Date()
  const days: { label: string; items: ActivityItem[] }[] = []
  for (const it of items) {
    const label = dayLabel(it.at, now)
    const last = days.at(-1)
    if (last?.label === label) last.items.push(it)
    else days.push({ label, items: [it] })
  }
  return days
}

function Activity({ items }: { items: ActivityItem[] }) {
  if (items.length === 0) return null
  return (
    <Section title="Activity">
      <div className="space-y-6">
        {byDay(items).map((day) => (
          <div key={day.label}>
            <p className="mb-1 px-2 text-xs font-medium text-muted-foreground">{day.label}</p>
            <ul aria-label={day.label} className="rounded-xl border bg-card p-1">
              {day.items.map((it) => (
                <li key={`${it.kind}-${it.id}`}>
                  <ActivityRow item={it} />
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </Section>
  )
}

const rowClass = 'flex items-start gap-3 rounded-lg px-2 py-2.5 hover:bg-secondary/50'

function ActivityRow({ item }: { item: ActivityItem }) {
  if (item.kind === 'attempt')
    return (
      <Link to="/history/$attemptId" params={{ attemptId: item.id }} className={rowClass}>
        <CheckCircle2Icon className="mt-0.5 size-4 shrink-0 text-primary" aria-hidden />
        <span className="min-w-0 flex-1 text-sm">
          {item.scope === 'topic' ? (
            <>
              Quizzed all of <span className="font-medium">{item.topicName}</span>
            </>
          ) : (
            <>
              Quizzed <span className="font-medium">{item.chapterTitle}</span>
            </>
          )}
          <span className="block truncate text-xs text-muted-foreground">
            {item.scope === 'chapter' && `${item.topicName} · `}
            {item.count} questions
          </span>
        </span>
        <ScorePill pct={item.scorePct} className="px-2 py-0.5 text-xs" />
      </Link>
    )
  return (
    <Link to="/chapters/$chapterId" params={{ chapterId: item.id }} className={rowClass}>
      {item.status === 'pending' ? (
        <Loader2Icon className="mt-0.5 size-4 shrink-0 animate-spin text-primary" aria-hidden />
      ) : item.status === 'failed' ? (
        <AlertTriangleIcon className="mt-0.5 size-4 shrink-0 text-destructive" aria-hidden />
      ) : (
        <FileUpIcon className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
      )}
      <span className="min-w-0 flex-1 text-sm">
        Uploaded <span className="font-medium">{item.chapterTitle}</span>
        <span className="block truncate text-xs text-muted-foreground">
          {item.topicName} ·{' '}
          {item.status === 'pending'
            ? 'generating questions…'
            : item.status === 'failed'
              ? 'generation failed'
              : `${String(item.questionCount)} questions`}
        </span>
      </span>
    </Link>
  )
}
