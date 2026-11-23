import { useRouter } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

import { Spinner } from './Spinner'

/**
 * The API runs on Render's free tier, which sleeps after 15 idle minutes and
 * takes ~30-60s to wake. Ping the providers endpoint and recover failed routes.
 */
export function ServerWakeBanner({ delayMs = 2500 }: { delayMs?: number }) {
  const router = useRouter()
  const [state, setState] = useState<'checking' | 'slow' | 'awake'>('checking')

  useEffect(() => {
    const ctrl = new AbortController()
    const timer = window.setTimeout(() => {
      setState((s) => (s === 'checking' ? 'slow' : s))
    }, delayMs)
    const ping = async (): Promise<void> => {
      for (let i = 0; i < 30 && !ctrl.signal.aborted; i++) {
        try {
          const res = await fetch('/api/auth/providers', { signal: ctrl.signal })
          if (res.ok) {
            setState('awake')
            // Route loaders may have exhausted their retries during the cold start.
            if (i > 0 && !ctrl.signal.aborted) await router.invalidate()
            return
          }
        } catch {
          // keep trying while the server boots
        }
        await new Promise((r) => window.setTimeout(r, 2000))
      }
    }
    void ping()
    return () => {
      ctrl.abort()
      window.clearTimeout(timer)
    }
  }, [delayMs, router])

  if (state !== 'slow') return null
  return (
    <div
      role="status"
      className="flex items-center justify-center gap-2 border-b border-primary/15 bg-secondary px-4 py-2 text-sm text-secondary-foreground"
    >
      <Spinner className="size-3" /> Waking up the server — this can take up to a minute after a
      period of inactivity.
    </div>
  )
}
