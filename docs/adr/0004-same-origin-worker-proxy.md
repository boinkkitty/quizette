# 4. Same-origin API through a Cloudflare Worker proxy

Status: accepted (2026-11-20). The session/CSRF paragraph was briefly superseded by ADR 0006 and is back in force per [ADR 0007](0007-revert-to-opaque-session-cookies.md). The Worker proxy itself is superseded by [ADR 0009](0009-single-binary-on-render.md); the session/CSRF design still applies.

## Context
Frontend on Cloudflare (`*.workers.dev`), API on Render (`*.onrender.com`). Cross-site session cookies are blocked by modern browsers' third-party-cookie rules, and CORS adds complexity.

## Decision
A Worker serves the SPA's static assets and proxies `/api/*` to Render (`worker/src/index.ts`), passing redirects through (`redirect: 'manual'`) for OAuth. The session is an opaque random token in an `HttpOnly; Secure; SameSite=Lax` cookie; only its SHA-256 is stored. Mutating requests must carry `X-Requested-With` (browsers can't add it cross-site without CORS, which is never enabled). Vite's dev proxy mirrors this locally.

## Consequences
One origin, first-party cookies, no CORS. Every API call adds a Worker hop (small). OAuth redirect URIs use the public (Worker) origin: `https://<app>/api/auth/{google,github}/callback`.
