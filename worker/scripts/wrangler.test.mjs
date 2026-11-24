import { test } from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, copyFileSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { spawnSync } from 'node:child_process'

function run(t, fileContents, override, args = ['deploy', '--dry-run']) {
  const directory = mkdtempSync(join(tmpdir(), 'quizette-worker-'))
  t.after(() => rmSync(directory, { recursive: true, force: true }))
  mkdirSync(join(directory, 'scripts'))
  copyFileSync(new URL('./wrangler.mjs', import.meta.url), join(directory, 'scripts/wrangler.mjs'))
  if (fileContents !== null) writeFileSync(join(directory, '.env'), fileContents)
  writeFileSync(join(directory, 'pnpm'), '#!/usr/bin/env node\nconsole.log(JSON.stringify(process.argv.slice(2)))\n', { mode: 0o755 })
  const env = { ...process.env, PATH: `${directory}:${process.env.PATH}` }
  delete env.API_ORIGIN
  if (override !== undefined) env.API_ORIGIN = override
  return spawnSync(process.execPath, [join(directory, 'scripts/wrangler.mjs'), ...args], {
    env,
    encoding: 'utf8',
  })
}

test('loads the worker dotenv file and forwards the binding and deployment options', (t) => {
  const result = run(t, '# Render\nAPI_ORIGIN="https://example.onrender.com"\n')
  assert.equal(result.status, 0, result.stderr)
  assert.deepEqual(JSON.parse(result.stdout), ['exec', 'wrangler', 'deploy', '--dry-run', '--var', 'API_ORIGIN:https://example.onrender.com'])
})

test('accepts a CI environment without a dotenv file', (t) => {
  const result = run(t, null, 'https://ci.onrender.com')
  assert.equal(result.status, 0, result.stderr)
  assert.ok(JSON.parse(result.stdout).includes('API_ORIGIN:https://ci.onrender.com'))
})

test('the environment takes precedence over the local file', (t) => {
  const result = run(t, 'API_ORIGIN=https://local.onrender.com\n', 'https://ci.onrender.com')
  assert.equal(result.status, 0, result.stderr)
  assert.ok(JSON.parse(result.stdout).includes('API_ORIGIN:https://ci.onrender.com'))
})

test('rejects missing or invalid origins before invoking Wrangler', (t) => {
  for (const origin of ['', 'not-a-url', 'ftp://example.com', 'https://example.com/api', 'https://user:password@example.com']) {
    const result = run(t, `API_ORIGIN=${origin}\n`)
    assert.equal(result.status, 1)
    assert.match(result.stderr, /Set API_ORIGIN/)
    assert.equal(result.stdout, '')
  }
})

test('uploads a preview version with the same binding', (t) => {
  const result = run(t, 'API_ORIGIN=https://example.onrender.com\n', undefined, ['versions', 'upload'])
  assert.equal(result.status, 0, result.stderr)
  assert.deepEqual(JSON.parse(result.stdout), ['exec', 'wrangler', 'versions', 'upload', '--var', 'API_ORIGIN:https://example.onrender.com'])
})
