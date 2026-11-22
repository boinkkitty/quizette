import { defineConfig, devices } from '@playwright/test'
import { existsSync } from 'node:fs'

// Settings come from .env.local (template: .env.example) locally, and from the workflow env in CI.
// Variables already set in the shell win over the file.
if (existsSync('.env.local')) process.loadEnvFile('.env.local')

function required(key: string): string {
  const value = process.env[key]
  if (value === undefined || value === '') {
    throw new Error(`${key} is not set. Copy e2e/.env.example to e2e/.env.local.`)
  }
  return value
}

const DATABASE_URL = required('DATABASE_URL')
const API_PORT = required('E2E_API_PORT')
const WEB_PORT = required('E2E_WEB_PORT')
const apiURL = `http://localhost:${API_PORT}`
const webURL = `http://localhost:${WEB_PORT}`
const ci = process.env['CI'] !== undefined

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: ci,
  retries: ci ? 1 : 0,
  reporter: ci ? [['github'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: webURL,
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: [
    {
      // Real Go API with the deterministic stub LLM (1s delay to exercise the polling UI).
      command: 'go run ./cmd/api',
      cwd: '../backend',
      url: `${apiURL}/readyz`,
      timeout: 180_000,
      reuseExistingServer: !ci,
      env: {
        DATABASE_URL,
        PORT: API_PORT,
        PUBLIC_URL: webURL,
        APP_ENV: 'test',
        LLM_PROVIDER: 'stub',
        STUB_LLM_DELAY: '1s',
        DAILY_UPLOAD_LIMIT: '1000',
        LOG_LEVEL: 'warn',
      },
    },
    {
      command: 'pnpm dev --strictPort',
      cwd: '../frontend',
      url: webURL,
      timeout: 60_000,
      env: { DEV_PORT: WEB_PORT, API_PROXY_TARGET: apiURL },
      reuseExistingServer: !ci,
    },
  ],
})
