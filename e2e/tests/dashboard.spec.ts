import { expect, test } from '@playwright/test'

import { expectNoOverflow, signUp, submitUnanswered, uploadSample } from './helpers'

test('the Dashboard leads a new account to Upload, then shows its uploads and quizzes', async ({
  page,
}) => {
  const mobile = { width: 390, height: 844 }
  const desktop = page.viewportSize() ?? { width: 1280, height: 720 }
  await signUp(page)
  await page.setViewportSize(mobile)
  const main = page.getByRole('main')
  await expect(main.getByRole('heading', { name: 'Turn a PDF into a quiz' })).toBeVisible()
  await expect(main.getByRole('link', { name: 'Upload a PDF' })).toHaveAttribute('href', '/upload')
  await expectNoOverflow(page)

  await page.setViewportSize(desktop) // the upload helper uses the sidebar
  await uploadSample(page, { topic: 'Biology', count: 5, title: 'Cells' })
  await page.goto('/quizzes?chapterId=' + page.url().split('/').at(-1))
  await page.getByRole('button', { name: 'Start 5-question quiz' }).click()
  await submitUnanswered(page, 5)

  await page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: 'Dashboard' }).click()
  await page.setViewportSize(mobile)
  await expect(main.getByRole('heading', { level: 1, name: 'Dashboard' })).toBeVisible()
  const tile = (label: string) => main.getByText(label, { exact: true }).locator('..')
  await expect(tile('Average score')).toContainText('0%')
  await expect(tile('Quizzes this week')).toContainText('1')
  await expect(tile('Questions answered')).toContainText('5')
  await expect(tile('Chapters quizzed')).toContainText('1 / 1')

  const biology = main.getByRole('list', { name: 'By topic' }).getByRole('listitem')
  await expect(biology).toHaveCount(1)
  await expect(biology).toContainText('Biology')
  await expect(biology).toContainText('1 of 1 chapters quizzed')
  await expect(biology.getByRole('link', { name: 'Quiz topic' })).toBeVisible()

  const today = main.getByRole('list', { name: 'Today' }).getByRole('link')
  await expect(today).toHaveCount(2)
  await expect(today.first()).toContainText('Quizzed Cells')
  await expect(today.first()).toContainText('0%')
  await expect(today.first()).toHaveAttribute('href', /^\/history\/[^/]+$/)
  await expect(today.last()).toContainText('Uploaded Cells')
  await expect(today.last()).toContainText('5 questions')
  await expectNoOverflow(page)

  await today.first().click()
  await expect(page).toHaveURL(/\/history\/[^/]+$/)
})
