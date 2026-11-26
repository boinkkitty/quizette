import { describe, expect, it } from 'vitest'

import { splitAcrossChapters } from './split'

// The same table as TestSplitAcrossChapters in backend/internal/domain/selection_test.go:
// the preview must show exactly what the server will draw.
describe('splitAcrossChapters', () => {
  it.each([
    ['even sizes', 10, [10, 10], [5, 5]],
    ['single', 5, [20], [5]],
    ['whole pool', 5, [3, 2], [3, 2]],
    ['uneven sizes', 10, [5, 10, 15], [2, 3, 5]],
    ['tie goes to the earlier chapter', 5, [7, 3], [4, 1]],
    ['chapter smaller than its share', 5, [1, 9], [1, 4]],
    ['several tiny chapters', 5, [1, 1, 1, 7], [1, 1, 0, 3]],
    ['tiny chapter gets nothing', 5, [1, 19], [0, 5]],
  ])('%s', (_name, n, sizes, want) => {
    expect(splitAcrossChapters(n, sizes)).toEqual(want)
  })
})
