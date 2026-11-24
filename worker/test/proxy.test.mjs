import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createServer } from 'node:http'

import worker, { WAKE_URL_HEADER } from '../src/index.ts'

// A stand-in for Render: /api/asleep mimics a suspended free-tier service.
async function startUpstream(t) {
  const server = createServer((req, res) => {
    if (req.url === '/api/asleep') {
      res.writeHead(502, { 'x-render-routing': 'no-deploy' }).end()
    } else if (req.url === '/api/crashed') {
      res.writeHead(502).end()
    } else {
      res.writeHead(200, { 'content-type': 'application/json' }).end('{"ok":true}')
    }
  })
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve))
  t.after(() => server.close())
  return `http://127.0.0.1:${server.address().port}`
}

function proxy(origin, path) {
  const env = { API_ORIGIN: origin, ASSETS: { fetch: () => new Response('asset') } }
  return worker.fetch(new Request(`https://app.example${path}`), env)
}

test('points the browser at Render when the service is asleep', async (t) => {
  const origin = await startUpstream(t)
  const res = await proxy(origin, '/api/asleep')
  assert.equal(res.status, 502)
  assert.equal(res.headers.get(WAKE_URL_HEADER), `${origin}/healthz`)
})

test('adds no wake URL to other responses', async (t) => {
  const origin = await startUpstream(t)
  for (const path of ['/api/crashed', '/api/ok']) {
    const res = await proxy(origin, path)
    assert.equal(res.headers.get(WAKE_URL_HEADER), null, path)
  }
  const ok = await proxy(origin, '/api/ok')
  assert.deepEqual(await ok.json(), { ok: true })
})
