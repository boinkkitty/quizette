import { expect, test } from '@playwright/test'

import { notAPdf, signUp, uploadSample } from './helpers'

// The stub LLM (backend/internal/adapters/llm/stub) cycles MCQ (answer "Option B"),
// True/False (answer "True") and MRQ (answers "A" and "E").

test('upload a PDF, take a quiz with partial MRQ credit, see history', async ({ page }) => {
  await signUp(page)
  await uploadSample(page, { topic: 'Biology', count: 10, title: 'Cells' })

  await expect(page.getByRole('heading', { name: 'Cells' })).toBeVisible()
  await expect(page.getByText('10 questions in this bank')).toBeVisible()
  const chapterURL = page.url()
  await page.getByLabel('Number of questions').selectOption('5')
  await page.getByRole('button', { name: 'Start quiz' }).click()
  await expect(page).toHaveURL(/\/quizzes\/run$/)

  // Answers question i the stub's way and returns the points it earns.
  const answer = async (i: number): Promise<number> => {
    await expect(page.getByText(`Question ${String(i + 1)} of 5`)).toBeVisible()
    if ((await page.getByRole('checkbox').count()) > 0) {
      await page.getByRole('checkbox', { name: 'A', exact: true }).check() // half credit
      return 0.5
    }
    if ((await page.getByRole('radio', { name: 'Option B' }).count()) > 0) {
      await page.getByRole('radio', { name: 'Option B' }).check()
      return 1
    }
    await page.getByRole('radio', { name: 'False' }).check() // wrong
    return 0
  }

  let expected = 0
  for (let i = 0; i < 2; i++) {
    expected += await answer(i)
    await page.getByRole('button', { name: 'Next' }).click()
  }

  // A reload keeps the quiz: same position, earlier answers still there.
  await page.reload()
  await expect(page.getByText('Question 3 of 5')).toBeVisible()
  await expect(page.getByText('2 answered')).toBeVisible()

  for (let i = 2; i < 5; i++) {
    expected += await answer(i)
    await page.getByRole('button', { name: i === 4 ? 'Submit quiz' : 'Next' }).click()
  }

  await expect(page).toHaveURL(/\/attempts\/[^/]+\/results$/)
  await expect(page.getByTestId('score')).toHaveText(`${String(expected)} / 5`)

  // Back from the results skips the finished quiz.
  await page.goBack()
  await expect(page).toHaveURL(chapterURL)
  const history = page.getByRole('table')
  await expect(history.getByRole('row')).toHaveCount(2) // header + 1 attempt
  await expect(history).toContainText(`${String(expected)} / 5`)

  // Topic page reflects the attempt.
  await page.getByRole('link', { name: '← Biology' }).click()
  await expect(page.getByText('10 questions · 1 attempts')).toBeVisible()

  // The sidebar's Quizzes and History pages list the chapter and the attempt.
  const nav = page.getByRole('navigation', { name: 'Main' })
  await nav.getByRole('link', { name: 'Quizzes' }).click()
  await page.getByRole('main').getByRole('button', { name: /Biology/ }).click()
  await expect(page.getByRole('button', { name: /Cells/ })).toContainText(/1 quiz · best \d+%/)

  // View questions: walk the whole bank with answers hidden until asked for.
  await page.goto(`${chapterURL}/questions`)
  await expect(page.getByText('Question 1 of 10')).toBeVisible()
  await expect(page.getByRole('radio')).toHaveCount(0)
  await page.getByRole('button', { name: 'Show answer' }).click()
  await expect(page.getByRole('button', { name: 'Hide answer' })).toBeVisible()
  await page.getByRole('button', { name: 'Next' }).click()
  await expect(page.getByText('Question 2 of 10')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Show answer' })).toBeVisible()

  await nav.getByRole('link', { name: 'History' }).click()
  const rows = page.getByRole('table').getByRole('row')
  await expect(rows).toHaveCount(2) // header + 1 attempt
  await expect(rows.nth(1)).toContainText('Cells')
  await expect(rows.nth(1)).toContainText(`${String(expected)} / 5`)
  await rows.nth(1).getByRole('link', { name: 'Review' }).click()
  await expect(page.getByTestId('score')).toHaveText(`${String(expected)} / 5`)
})

test('upload validation', async ({ page }) => {
  await signUp(page)
  await page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: 'Upload' }).click()
  await page.getByLabel('PDF file').setInputFiles(notAPdf)
  await expect(page.getByText('Only PDF files are supported.')).toBeVisible()
  await page.getByRole('button', { name: 'Generate questions' }).click()
  await expect(page.getByText('Choose a PDF to upload.')).toBeVisible()
})

test("users cannot see each other's chapters", async ({ browser }) => {
  const alice = await browser.newPage()
  await signUp(alice)
  await uploadSample(alice, { topic: 'Private', count: 5 })
  const chapterUrl = alice.url()

  const eve = await browser.newPage()
  await signUp(eve)
  await eve.goto(chapterUrl)
  await expect(eve.getByText("We couldn't find that")).toBeVisible()
  await expect(eve.getByRole('heading', { name: 'Your topics' })).toHaveCount(0)
})
