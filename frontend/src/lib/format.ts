import type { QuestionType } from '../api/schemas'

/** Formats points like "2.5" or "0.67" (no trailing zeros). */
export function formatPoints(points: number): string {
  return String(Math.round(points * 100) / 100)
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
