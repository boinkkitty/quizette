import { expect, test } from '@playwright/test'

import { signUp, uniqueEmail } from './helpers'

test('sign up, log out, log back in', async ({ page }) => {
  const email = await signUp(page)
  await page.getByRole('button', { name: 'Sign out' }).click()
  await expect(page.getByRole('heading', { name: 'Log in to Quizette' })).toBeVisible()

  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password').fill('wrong password')
  await page.getByRole('button', { name: 'Log in' }).click()
  await expect(page.getByText('invalid email or password')).toBeVisible()

  await page.getByLabel('Password').fill('correct horse battery')
  await page.getByRole('button', { name: 'Log in' }).click()
  await expect(page.getByRole('heading', { level: 1, name: 'Dashboard' })).toBeVisible()
})

test('protected pages redirect to login and back', async ({ page }) => {
  await page.goto('/upload')
  await expect(page).toHaveURL(/\/login\?redirect=%2Fupload/)
  await page.getByRole('link', { name: 'Sign up' }).click()
  await page.getByLabel('Email').fill(uniqueEmail())
  await page.getByLabel('Password').fill('correct horse battery')
  await page.getByRole('button', { name: 'Sign up' }).click()
  await expect(page.getByRole('heading', { name: 'Upload a PDF' })).toBeVisible()
})

test('duplicate email is rejected', async ({ page, context }) => {
  const email = await signUp(page)
  await context.clearCookies()
  await page.goto('/signup')
  await page.getByLabel('Email').fill(email.toUpperCase())
  await page.getByLabel('Password').fill('another password')
  await page.getByRole('button', { name: 'Sign up' }).click()
  await expect(page.getByText('an account with this email already exists')).toBeVisible()
})

test('session lives in an HttpOnly cookie, survives a reload and ends on logout', async ({ page }) => {
  const apiAuth: string[] = []
  page.on('request', (req) => {
    if (new URL(req.url()).pathname === '/api/dashboard') apiAuth.push(req.headers()['authorization'] ?? '')
  })
  await signUp(page)

  const cookies = await page.context().cookies()
  const session = cookies.find((c) => c.name === 'qz_session')
  expect(session).toMatchObject({ httpOnly: true, sameSite: 'Lax', path: '/' })
  expect(await page.evaluate(() => document.cookie)).not.toContain('qz_session')

  await page.reload()
  await expect(page.getByRole('heading', { level: 1, name: 'Dashboard' })).toBeVisible()
  expect(apiAuth.length).toBeGreaterThanOrEqual(2)
  for (const a of apiAuth) expect(a).toBe('') // cookie only, no Authorization header

  await page.getByRole('button', { name: 'Sign out' }).click()
  await expect(page.getByRole('heading', { name: 'Log in to Quizette' })).toBeVisible()
  expect((await page.context().cookies()).some((c) => c.name === 'qz_session')).toBe(false)
  await page.goto('/')
  await expect(page).toHaveURL(/\/login/)
})
