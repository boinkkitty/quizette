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
  await expect(page.getByRole('heading', { level: 1, name: 'Dashboard' })).toBeVisible()
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
  await page.getByRole('link', { name: 'Go to chapter', exact: true }).click()
  // Wait for the chapter page: navigating on while its loader runs would cancel the
  // navigation and leave the upload page as it is.
  await expect(page).toHaveURL(/\/chapters\/[^/]+$/)
  if (opts.title !== undefined) {
    await expect(page.getByRole('heading', { name: opts.title })).toBeVisible()
  }
}

/** Fails if the page scrolls sideways. */
export async function expectNoOverflow(page: Page): Promise<void> {
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  )
  expect.soft(overflow, `horizontal overflow (px) on ${page.url()}`).toBeLessThanOrEqual(0)
}

/**
 * Fails if the page scrolls sideways or a control in main is smaller than 44 × 44 px. Links inside a
 * sentence (a `p`) are exempt, as WCAG 2.5.8 exempts inline targets, and so are disabled and
 * visually hidden controls, and a control whose label is big enough. Wait for the page to settle first: a control that is leaving is 0 × 0.
 */
export async function expectMobileFriendly(page: Page): Promise<void> {
  await expectNoOverflow(page)
  const small = await page
    .getByRole('main')
    .locator('button:visible, a:visible, input:visible')
    .evaluateAll((controls) =>
      controls.flatMap((c) => {
        if (c.tagName === 'A' && c.closest('p') !== null) return []
        if (c.matches(':disabled')) return []
        const { width, height } = c.getBoundingClientRect()
        // Visually hidden controls (sr-only) behind a larger visible label.
        if (width <= 1 || height <= 1) return []
        const big = (r: DOMRect) => Math.round(r.width) >= 44 && Math.round(r.height) >= 44
        if (big(c.getBoundingClientRect())) return []
        // A small radio or checkbox is fine when its label, which also toggles it, is the target.
        const labels = 'labels' in c ? [...((c as HTMLButtonElement).labels ?? [])] : []
        if (labels.some((l) => big(l.getBoundingClientRect()))) return []
        const name =
          c.getAttribute('aria-label') ?? (c.textContent?.trim() || c.outerHTML.slice(0, 80))
        return [`"${name}" ${String(Math.round(width))}×${String(Math.round(height))}`]
      }),
    )
  expect.soft(small, `controls under 44 × 44 px on ${page.url()}`).toEqual([])
}

/** Clicks through every question without answering and submits anyway. */
export async function submitUnanswered(page: Page, count: number): Promise<void> {
  for (let i = 1; i < count; i++) {
    await expect(page.getByText(`Question ${String(i)} of ${String(count)}`)).toBeVisible()
    await page.getByRole('button', { name: 'Next' }).click()
  }
  await page.getByRole('button', { name: 'Submit quiz' }).click()
  await page.getByRole('button', { name: 'Submit anyway' }).click()
  await expect(page).toHaveURL(/\/history\/[^/]+$/)
  await expect(page.getByTestId('score')).toHaveText(`0 / ${String(count)}`)
}
