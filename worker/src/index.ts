/**
 * Quizette edge worker: serves the SPA from static assets and proxies /api/*
 * to the Go API on Render. Browser and API share one origin, so the HttpOnly
 * session cookie is first-party and no CORS is needed. Cookie and Set-Cookie
 * headers pass through unchanged.
 */
export interface Env {
  API_ORIGIN: string // e.g. https://quizette-api.onrender.com
  ASSETS: Fetcher
}

/** Response header naming a Render URL the browser should request directly to wake the API. */
export const WAKE_URL_HEADER = 'X-Wake-Url'

const HOP_BY_HOP =['connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization', 'te', 'trailer', 'transfer-encoding', 'upgrade']

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url)
    if (!url.pathname.startsWith('/api/')) return env.ASSETS.fetch(request)

    const target = new URL(url.pathname + url.search, env.API_ORIGIN)
    const headers = new Headers(request.headers)
    for (const h of HOP_BY_HOP) headers.delete(h)
    headers.set('X-Forwarded-Host', url.host)
    headers.set('X-Forwarded-Proto', url.protocol.replace(':', ''))
    const ip = request.headers.get('CF-Connecting-IP')
    if (ip !== null) headers.set('X-Forwarded-For', ip)

    const upstream = await fetch(target, {
      method: request.method,
      headers,
      body: request.method === 'GET' || request.method === 'HEAD' ? null : request.body,
      redirect: 'manual', // pass OAuth redirects through to the browser
    })
    const res = new Response(upstream.body, upstream)
    res.headers.set('Cache-Control', 'no-store')
    // A sleeping free-tier Render service answers Worker requests with an instant
    // 502 (x-render-routing: no-deploy) and does not start; only a request made
    // directly to Render wakes it. Tell the browser where to send that request.
    if (upstream.status === 502 && upstream.headers.get('x-render-routing')?.includes('no-deploy')) {
      res.headers.set(WAKE_URL_HEADER, new URL('/healthz', env.API_ORIGIN).href)
    }
    return res
  },
} satisfies ExportedHandler<Env>
