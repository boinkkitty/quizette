import { loadEnvFile } from 'node:process'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const workerDirectory = fileURLToPath(new URL('../', import.meta.url))
try {
  loadEnvFile(new URL('../.env', import.meta.url))
} catch (error) {
  // CI may supply API_ORIGIN directly instead of using a local file.
  if (error.code !== 'ENOENT') throw error
}

const origin = process.env.API_ORIGIN
try {
  const url = new URL(origin)
  if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password ||
      url.pathname !== '/' || url.search || url.hash) throw new Error('Invalid origin')
} catch {
  console.error('Set API_ORIGIN to an HTTP(S) origin in worker/.env or the environment (for example https://your-service.onrender.com).')
  process.exit(1)
}

const [command, ...args] = process.argv.slice(2)
if (!['dev', 'deploy', 'versions'].includes(command)) {
  console.error('Usage: node scripts/wrangler.mjs <dev|deploy|versions> [Wrangler options]')
  process.exit(1)
}

const result = spawnSync('pnpm', ['exec', 'wrangler', command, ...args, '--var', `API_ORIGIN:${origin}`], {
  cwd: workerDirectory,
  stdio: 'inherit',
})
if (result.error) console.error(result.error.message)
process.exit(result.status ?? 1)
