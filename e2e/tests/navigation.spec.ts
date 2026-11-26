import { expect, test } from '@playwright/test'

import { signUp, uploadSample } from './helpers'

// Each page has one fixed parent, and its back link always goes there.

test('browse Topics to a chapter and its questions, then quiz the chapter', async ({ page }) => {
  await signUp(page)
  await uploadSample(page, { topic: 'Biology', count: 5, title: 'Cells' })

  const nav = page.getByRole('navigation', { name: 'Main' })
  await nav.getByRole('link', { name: 'Topics' }).click()
  await expect(page.getByRole('heading', { name: 'Topics' })).toBeVisible()
  await page.getByRole('link', { name: /Biology/ }).click()
  await expect(page.getByRole('heading', { name: 'Biology' })).toBeVisible()
  await page.getByRole('link', { name: /Cells/ }).click()
  await expect(page.getByRole('heading', { name: 'Cells' })).toBeVisible()
  const chapterURL = page.url()

  await page.getByRole('link', { name: 'View questions' }).click()
  await expect(page.getByText('Question 1 of 5')).toBeVisible()
  for (let i = 1; i < 5; i++) await page.getByRole('button', { name: 'Next' }).click()
  await page.getByRole('link', { name: 'Done' }).click()
  await expect(page).toHaveURL(chapterURL)

  // Quiz this chapter opens the picker at step 3 with the chapter chosen.
  await page.getByRole('link', { name: 'Quiz this chapter' }).click()
  await expect(page).toHaveURL(/\/quizzes\?chapterId=/)
  await expect(page.getByRole('slider', { name: 'Number of questions' })).toHaveValue('5')
  await expect(page.getByRole('main')).toContainText('Cells')
  await expect(page.getByRole('button', { name: 'Start 5-question quiz' })).toBeVisible()

  // The back links lead up the fixed parents.
  await page.goto(chapterURL)
  await page.getByRole('link', { name: '← Biology' }).click()
  await page.getByRole('link', { name: '← Topics' }).click()
  await expect(page).toHaveURL(/\/topics$/)
})
