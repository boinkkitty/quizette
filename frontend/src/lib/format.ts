import type { ChapterStatus, QuestionType } from '../api/schemas'

/** Formats points like "2.5" or "0.67" (no trailing zeros). */
export function formatPoints(points: number): string {
  return String(Math.round(points * 100) / 100)
}

/** Formats a count with its noun, like "1 attempt" or "3 chapters". many defaults to one + "s". */
export function plural(n: number, one: string, many = `${one}s`): string {
  return `${String(n)} ${n === 1 ? one : many}`
}

/**
 * Labels a topic's chapters, like "3 chapters · 1 failed", so every page counts them the same way.
 * The total includes chapters still generating or failed.
 */
export function chapterCountLabel(chapters: readonly { status: ChapterStatus }[]): string {
  const failed = chapters.filter((c) => c.status === 'failed').length
  const total = plural(chapters.length, 'chapter')
  return failed === 0 ? total : `${total} · ${String(failed)} failed`
}

export function formatPct(pct: number): string {
  return `${String(Math.round(pct))}%`
}

export function formatBytes(n: number): string {
  if (n < 1024) return `${String(n)} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}

export const questionTypeLabel: Record<QuestionType, string> = {
  mcq: 'Multiple choice',
  true_false: 'True / False',
  mrq: 'Select all that apply',
}

/** Multiples of 5 from 5 up to max (inclusive). */
export function countOptions(max: number): number[] {
  const out: number[] = []
  for (let n = 5; n <= max; n += 5) out.push(n)
  return out
}
