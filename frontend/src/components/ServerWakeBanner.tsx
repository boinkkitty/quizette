import { useEffect, useState } from 'react'

import { Button } from './ui/button'
import { Spinner } from './Spinner'

/** Set by the Cloudflare Worker on a 502 from a sleeping Render service (worker/src/index.ts). */
const WAKE_URL_HEADER = 'X-Wake-Url'

/** How often to re-send the direct wake request while Render boots. */
const REWAKE_MS = 30_000

type State = 'checking' | 'slow' | 'awake' | 'failed'

/**
 * The API runs on Render's free tier, which sleeps after 15 idle minutes. A sleeping
 * service answers requests proxied through the Worker with an instant 502 and never
 * starts; only a request sent straight to Render wakes it (~15-60s). So this pings the
 * API through the Worker and, when the Worker names a wake URL, requests it directly.
 *
 * It renders outside the router so it starts at page load, not after the route loaders
 * have failed. onAwake runs once the API answers after at least one failure, so routes
 * that failed during the cold start can reload.
 */
export function ServerWakeBanner(props: WakeCheckProps) {
  const [run, setRun] = useState(0)
  // A new key restarts the check from scratch.
  return <WakeCheck key={run} {...props} onRetry={() => setRun((n) => n + 1)} />
}

interface WakeCheckProps {
  onAwake: () => Promise<void>
  delayMs?: number
  intervalMs?: number
  maxWaitMs?: number
}

function WakeCheck({
  onAwake,
  onRetry,
  delayMs = 2500,
  intervalMs = 2000,
  maxWaitMs = 120_000,
}: WakeCheckProps & { onRetry: () => void }) {
  const [state, setState] = useState<State>('checking')

  useEffect(() => {
    const ctrl = new AbortController()
    const timer = window.setTimeout(() => {
      setState((s) => (s === 'checking' ? 'slow' : s))
    }, delayMs)
    const ping = async (): Promise<void> => {
      const started = Date.now()
      let lastWake = -Infinity
      for (let i = 0; !ctrl.signal.aborted; i++) {
        try {
          const res = await fetch('/api/auth/providers', { signal: ctrl.signal })
          if (res.ok) {
            setState('awake')
            if (i > 0 && !ctrl.signal.aborted) await onAwake()
            return
          }
          const wakeURL = res.headers.get(WAKE_URL_HEADER)
          if (wakeURL !== null && Date.now() - lastWake >= REWAKE_MS) {
            lastWake = Date.now()
            // Opaque cross-origin request: only its arrival at Render matters.
            fetch(wakeURL, { mode: 'no-cors', signal: ctrl.signal }).catch(() => {})
          }
        } catch {
          // Network error or abort: keep trying while the server boots.
        }
        if (Date.now() - started >= maxWaitMs) {
          if (!ctrl.signal.aborted) setState('failed')
          return
        }
        await new Promise((r) => window.setTimeout(r, intervalMs))
      }
    }
    void ping()
    return () => {
      ctrl.abort()
      window.clearTimeout(timer)
    }
  }, [delayMs, intervalMs, maxWaitMs, onAwake])

  if (state === 'checking' || state === 'awake') return null
  return (
    <div
      role="status"
      className="flex items-center justify-center gap-2 border-b border-primary/15 bg-secondary px-4 py-2 text-sm text-secondary-foreground"
    >
      {state === 'slow' ? (
        <>
          <Spinner className="size-3" /> Waking up the server — this can take up to a minute after a
          period of inactivity.
        </>
      ) : (
        <>
          The server is taking longer than usual to start.
          <Button size="sm" variant="outline" onClick={onRetry}>
            Try again
          </Button>
        </>
      )}
    </div>
  )
}
