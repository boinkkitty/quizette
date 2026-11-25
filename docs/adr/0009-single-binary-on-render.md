# 9. Serve the SPA from the Go binary on Render

Status: accepted (2026-11-25). Supersedes the Worker proxy of [ADR 0004](0004-same-origin-worker-proxy.md); its session/CSRF design still applies.

## Context
A Cloudflare Worker served the SPA and proxied `/api/*` to the API on Render's free tier, which sleeps after 15 idle minutes. Render does not start a sleeping service for requests that come from a Worker: it answers them with `502 no-deploy`, `429 hibernate-rate-limited` or a Cloudflare bot challenge (`cf-mitigated: challenge`), tested 2026-11-25. Only a request from the browser itself woke it, so the frontend needed a wake banner, an `X-Wake-Url` header naming the Render URL, and route reloads, and the Worker could neither hide nor shorten the cold start. The Worker also meant a second deploy, CI job and set of secrets.

## Decision
One deployable. The Docker build (root `Dockerfile`) builds the frontend, copies it into `backend/web/dist` and embeds it with `go:embed`. The HTTP adapter serves it for every path outside `/api`: existing files as-is (hashed `assets/*` cached for a year, everything else revalidated), `index.html` for extensionless paths so client-side routes work, and 404 for missing files. Render builds this image and deploys each `main` commit once its GitHub checks pass. Local development is unchanged: Vite serves the frontend and proxies `/api`, and a binary built without the frontend serves the API only.

## Consequences
- Browser and API still share one origin, so cookies stay first-party and no CORS is needed. One deploy keeps frontend and API in step.
- A cold start now blocks the page itself: the first visit after 15 idle minutes waits ~15–60 s with a blank tab. Accepted for now; an always-on plan removes it.
- No edge CDN: static files come from Render's region, compressed by Render's edge.
- A frontend-only change rebuilds and restarts the API, which drops any in-flight question generation.
