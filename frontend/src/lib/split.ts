/**
 * Splits n questions across chapters in proportion to their sizes (question counts) with
 * largest-remainder rounding, and returns how many to draw from each, in the order of sizes.
 * A remainder tie goes to the earlier chapter, so pass chapters in the server's order
 * (creation order). Assumes 0 <= n <= the sum of sizes.
 *
 * Mirrors SplitAcrossChapters in backend/internal/domain/selection.go: a change there must be
 * made here too. Integer quotient and remainder keep it free of floating-point rounding.
 */
export function splitAcrossChapters(n: number, sizes: readonly number[]): number[] {
  const total = sizes.reduce((s, x) => s + x, 0)
  if (total === 0) return sizes.map(() => 0)
  const out = sizes.map((s) => Math.floor((n * s) / total))
  const rem = sizes.map((s) => (n * s) % total)
  const given = out.reduce((s, x) => s + x, 0)
  // Array.prototype.sort is stable, so equal remainders keep chapter order.
  const order = sizes.map((_, i) => i).sort((a, b) => (rem[b] ?? 0) - (rem[a] ?? 0))
  for (const i of order.slice(0, n - given)) out[i] = (out[i] ?? 0) + 1
  return out
}
