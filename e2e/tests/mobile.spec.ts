import { expect, test } from '@playwright/test'

import { expectMobileFriendly, expectNoOverflow, signUp, submitUnanswered, uploadSample } from './helpers'

// Every page at 390 px: nothing scrolls sideways and every control in main is a 44 px tap target.

test.use({ viewport: { width: 390, height: 844 } })

test('logged-out pages fit a phone', async ({ page }) => {
  for (const path of ['/login', '/signup']) {
    await page.goto(path)
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
    await expectNoOverflow(page)
  }
})

test('every signed-in page fits a phone and has 44 px tap targets', async ({ page }) => {
  test.setTimeout(120_000)
  await signUp(page)
  await page.goto('/')
  await expectMobileFriendly(page)

  await page.goto('/topics')
  await expect(page.getByRole('heading', { level: 1, name: 'Topics' })).toBeVisible()
  await expectMobileFriendly(page)

  // uploadSample uses the sidebar, which phones fold away.
  await page.setViewportSize({ width: 1280, height: 800 })
  await uploadSample(page, {
    topic: 'A topic with a rather long name for a phone',
    count: 5,
    title: 'Cells and the long chapter title that wraps',
  })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.reload() // measure the phone layout, not the desktop one mid-resize
  await expect(page.getByRole('link', { name: 'Quiz this chapter' })).toBeVisible()
  const chapterURL = page.url()
  await expectMobileFriendly(page)

  // The quiz runner, from the chapter page's own entry point.
  await page.getByRole('link', { name: 'Quiz this chapter' }).click()
  await page.getByRole('button', { name: /^Start \d+-question quiz$/ }).click()
  await expect(page).toHaveURL(/\/quizzes\/run$/)
  await expect(page.getByText('Question 1 of 5')).toBeVisible()
  await expectMobileFriendly(page)
  await submitUnanswered(page, 5)
  await expectMobileFriendly(page) // Results

  // The Dashboard and Topics again, now with a topic, a chapter and a quiz on them.
  await page.goto('/')
  await expect(page.getByRole('list', { name: 'By topic' })).toBeVisible()
  await expectMobileFriendly(page)
  await page.goto('/topics')
  await expect(page.getByRole('button', { name: /^Delete topic / })).toBeVisible()
  await expectMobileFriendly(page)

  await page.goto('/history')
  await expect(page.getByRole('heading', { level: 1, name: 'History' })).toBeVisible()
  await expectMobileFriendly(page)

  // The chapter page now has an attempt table.
  await page.goto(chapterURL)
  await expect(page.getByRole('table')).toBeVisible()
  await expectMobileFriendly(page)

  await page.getByRole('link', { name: /View questions/ }).click()
  await expect(page.getByText('Question 1 of 5')).toBeVisible()
  await expectMobileFriendly(page)
  await page.getByRole('button', { name: 'Show answer' }).click()
  await page.getByRole('button', { name: 'Next' }).click()
  await expectMobileFriendly(page)

  await page.getByRole('link', { name: /^←/ }).click()
  await page.getByRole('link', { name: /^←/ }).click()
  await expect(page).toHaveURL(/\/topics\/[^/]+$/)
  await expectMobileFriendly(page)

  for (const path of ['/upload', '/quizzes']) {
    await page.goto(path)
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
    await expectMobileFriendly(page)
  }
})
