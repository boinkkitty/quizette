import { describe, expect, it } from 'vitest'

import { countOptions, formatBytes, formatPct, formatPoints, plural } from './format'
import { MAX_UPLOAD_BYTES, validateFile } from './upload'

describe('format', () => {
  it('formats points without trailing zeros', () => {
    expect(formatPoints(2.5)).toBe('2.5')
    expect(formatPoints(0.6667)).toBe('0.67')
    expect(formatPoints(3)).toBe('3')
  })
  it('formats percentages and bytes', () => {
    expect(formatPct(66.666)).toBe('67%')
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(2048)).toBe('2 KB')
    expect(formatBytes(3.5 * 1024 * 1024)).toBe('3.5 MB')
  })
  it('pluralises count labels', () => {
    expect(plural(1, 'attempt')).toBe('1 attempt')
    expect(plural(0, 'attempt')).toBe('0 attempts')
    expect(plural(3, 'chapter')).toBe('3 chapters')
    expect(plural(1, 'quiz', 'quizzes')).toBe('1 quiz')
    expect(plural(2, 'quiz', 'quizzes')).toBe('2 quizzes')
  })
  it('lists multiples of five up to max', () => {
    expect(countOptions(30)).toEqual([5, 10, 15, 20, 25, 30])
    expect(countOptions(12)).toEqual([5, 10])
    expect(countOptions(4)).toEqual([])
  })
})

describe('validateFile', () => {
  const file = (name: string, type: string, size: number) => {
    const f = new File(['x'], name, { type })
    Object.defineProperty(f, 'size', { value: size })
    return f
  }
  it.each([
    ['notes.pdf', 'application/pdf', 1000, undefined],
    ['NOTES.PDF', '', 1000, undefined],
    ['notes.docx', 'application/msword', 1000, 'Only PDF files are supported.'],
    ['empty.pdf', 'application/pdf', 0, 'This file is empty.'],
    ['big.pdf', 'application/pdf', MAX_UPLOAD_BYTES + 1, 'PDFs must be 15 MB or smaller.'],
  ])('%s', (name, type, size, want) => {
    expect(validateFile(file(name, type, size))).toBe(want)
  })
})
