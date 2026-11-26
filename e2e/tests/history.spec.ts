import { expect, test } from '@playwright/test'

import {
  expectMobileFriendly,
  expectNoOverflow,
  signUp,
  submitUnanswered,
  uploadSample,
} from './helpers'

test('a topic quiz opens in Results, filters by chapter, and is listed in History on mobile', async ({
  page,
}) => {
  await signUp(page)
  await uploadSample(page, { topic: 'Biology', count: 10, title: 'Cells' })
  await uploadSample(page, { topic: 'Biology', count: 10, title: 'Genetics' })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/quizzes')
  await page.getByRole('button', { name: /Biology/ }).click()
  await page.getByRole('button', { name: /Whole topic: Biology/ }).click()
  await page.getByRole('button', { name: 'Start 10-question quiz' }).click()
  await submitUnanswered(page, 10)

  // Results: the server splits 10 over two 10-question chapters 5 / 5.
  await expect(page.getByRole('heading', { name: 'Biology' })).toBeVisible()
  await expect(page.getByText(/^Whole-topic quiz · 2 chapters · /)).toBeVisible()
  const chips = page.getByRole('group', { name: 'Chapters' }).getByRole('button')
  // Chapters are listed in the order they first appear in the shuffled quiz.
  await expect(chips).toHaveCount(3)
  await expect(chips.first()).toHaveText('All chapters')
  expect((await chips.allTextContents()).slice(1).sort()).toEqual(['Cells0%', 'Genetics0%'])
  const answers = page.getByRole('list', { name: 'Answers' }).locator(':scope > li')
  await expect(answers).toHaveCount(10)
  await expectNoOverflow(page)

  await chips.filter({ hasText: 'Genetics' }).click()
  await expect(answers).toHaveCount(5)
  await expect(page.getByRole('link', { name: 'Open chapter →' })).toBeVisible()
  // Questions keep their numbers from the quiz: the two chapters' together are 1–10.
  const numbers = () =>
    answers.evaluateAll((items) =>
      items.map((li) => Number(/Question (\d+)/.exec(li.textContent ?? '')?.[1])),
    )
  const genetics = await numbers()
  await chips.filter({ hasText: 'Cells' }).click()
  await expect(answers).toHaveCount(5)
  const cells = await numbers()
  expect([...genetics, ...cells].sort((a, b) => a - b)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10])
  await expectNoOverflow(page)

  await page
    .getByRole('navigation', { name: 'Result actions' })
    .getByRole('link', { name: 'History' })
    .click()
  await expect(page).toHaveURL(/\/history$/)
  const row = page.getByRole('list', { name: 'Attempts' }).getByRole('link')
  await expect(row).toHaveCount(1)
  await expect(row).toContainText('All of Biology')
  await expect(row).toContainText('2 chapters · 10 questions')
  await expect(row).toContainText('0%')
  await expectMobileFriendly(page)
})
