import { expect, test } from '@playwright/test'

import { expectMobileFriendly, signUp, submitUnanswered, uploadSample } from './helpers'

test('quiz a whole topic, then one chapter, from the three-step picker on mobile', async ({
  page,
}) => {
  await signUp(page)
  await uploadSample(page, { topic: 'Biology', count: 10, title: 'Cells' })
  await uploadSample(page, { topic: 'Biology', count: 10, title: 'Genetics' })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/quizzes')

  // Step 1: topic.
  const biology = page.getByRole('button', { name: /Biology/ })
  await expect(biology).toContainText('2 chapters · 20 questions')
  await expect(biology).toContainText('Not quizzed yet')
  await expectMobileFriendly(page)
  await biology.click()

  // Step 2: what to quiz.
  await expect(page.getByRole('button', { name: /Genetics/ })).toBeVisible()
  await expectMobileFriendly(page)
  await page.getByRole('button', { name: /Whole topic: Biology/ }).click()

  // Step 3: how many. The server splits 10 across two 10-question chapters 5 / 5.
  const slider = page.getByRole('slider', { name: 'Number of questions' })
  await expect(slider).toHaveValue('10')
  await expect(slider).toHaveAttribute('max', '20')
  const split = page.getByRole('list', { name: 'Where they come from' }).getByRole('listitem')
  await expect(split).toHaveText(['Cells5', 'Genetics5'])
  await expectMobileFriendly(page)
  await page.getByRole('button', { name: 'Start 10-question quiz' }).click()

  await expect(page).toHaveURL(/\/quizzes\/run$/)
  await expect(page.getByRole('heading', { name: 'All of Biology' })).toBeVisible()
  await submitUnanswered(page, 10)

  // The topic now has a quiz on record; quiz one chapter the same way.
  await page.goto('/quizzes')
  await expect(biology).toContainText('1 quiz · last 0% · best 0%')
  await biology.click()
  await page.getByRole('button', { name: /Cells/ }).click()
  await expect(slider).toHaveValue('10')
  await expect(page.getByRole('list', { name: 'Where they come from' })).toHaveCount(0)
  await expectMobileFriendly(page)
  await page.getByRole('button', { name: 'Start 10-question quiz' }).click()

  await expect(page.getByRole('heading', { name: 'Cells' })).toBeVisible()
  await submitUnanswered(page, 10)
})
