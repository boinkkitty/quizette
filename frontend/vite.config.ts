import tailwindcss from '@tailwindcss/vite'
import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath } from 'node:url'
import { defineConfig, loadEnv } from 'vite'

// Dev-server settings come from .env.local (template: .env.example). They are read here only,
// not exposed to the client bundle (no VITE_ prefix). Shell variables override the file.
function required(env: Record<string, string>, key: string): string {
  const value = env[key]
  if (value === undefined || value === '') {
    throw new Error(`${key} is not set. Copy frontend/.env.example to frontend/.env.local.`)
  }
  return value
}

export default defineConfig(({ command, mode }) => {
  const plugins = [
    tanstackRouter({ target: 'react', autoCodeSplitting: true }),
    react(),
    tailwindcss(),
  ]
  const resolve = { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } }
  if (command === 'build') return { plugins, resolve }

  const env = loadEnv(mode, process.cwd(), '')
  return {
    plugins,
    resolve,
    server: {
      port: Number(required(env, 'DEV_PORT')),
      // Same-origin in dev too: mirrors the Cloudflare Worker proxy in production.
      proxy: { '/api': required(env, 'API_PROXY_TARGET') },
    },
  }
})
