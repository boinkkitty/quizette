import { randomUUID } from 'node:crypto'
import { fileURLToPath } from 'node:url'

import { expect, type Page } from '@playwright/test'

export const samplePdf = fileURLToPath(new URL('../fixtures/sample.pdf', import.meta.url))
export const notAPdf = fileURLToPath(new URL('../fixtures/not-a-pdf.txt', import.meta.url))

export function uniqueEmail(): string {
  return `e2e-${randomUUID()}@example.com`
}

export async function signUp(page: Page, email = uniqueEmail(), password = 'correct horse battery'): Promise<string> {
  await page.goto('/signup')
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign up' }).click()
  await expect(page.getByRole('heading', { name: 'Your topics' })).toBeVisible()
  return email
}

/** Uploads the sample PDF into a new topic and waits for the bank to be ready. */
export async function uploadSample(page: Page, opts: { topic: string; count: number; title?: string }): Promise<void> {
  await page.getByRole('navigation', { name: 'Main' }).getByRole('link', { name: 'Upload' }).click()
  await page.getByLabel('PDF file').setInputFiles(samplePdf)
  const topicSelect = page.getByLabel('Topic', { exact: true })
  if ((await topicSelect.locator('option', { hasText: opts.topic }).count()) > 0) {
    await topicSelect.selectOption({ label: opts.topic })
  } else {
    await topicSelect.selectOption({ label: '+ New topic…' })
    await page.getByLabel('New topic name').fill(opts.topic)
  }
  if (opts.title !== undefined) await page.getByLabel('Chapter title').fill(opts.title)
  await page.getByLabel('Questions to generate').selectOption(String(opts.count))
  await page.getByRole('button', { name: 'Generate questions' }).click()
  await expect(page.getByText(`Generating ${String(opts.count)} questions`)).toBeVisible()
  await expect(page.getByText('Your question bank is ready')).toBeVisible({ timeout: 30_000 })
  await page.getByRole('link', { name: 'Go to chapter →' }).click()
}
