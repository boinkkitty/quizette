# 6. JWT access tokens with a rotating refresh cookie

Status: **superseded** by [ADR 0007](0007-revert-to-opaque-session-cookies.md) (2026-11-22). Kept for the record; this design is no longer implemented.

## Context
Login used an opaque session token in an `HttpOnly` cookie, looked up in Postgres on every request, with an `X-Requested-With` header guarding every mutation against CSRF. We want the API to authenticate requests with `Authorization: Bearer` (stateless verification, no per-request DB hit, the same contract for future non-browser clients) without storing a long-lived credential where page JavaScript can read it.

## Decision
- **Access token:** an HS256 JWT (`adapters/jwt`, behind the `app.AccessTokens` port). Claims: `sub` = user ID, `iss=quizette`, `aud=quizette-api`, `iat`, `exp` (15 minutes, `ACCESS_TOKEN_TTL`). The signing key is `JWT_SECRET` (≥ 32 bytes, required in production). Verification pins the algorithm to HS256 and requires `exp`, issuer and audience.
- **Where it lives:** in browser memory only (`frontend/src/auth/tokenStore.ts`), never in `localStorage`. It is sent as `Authorization: Bearer` on every API call.
- **Refresh token:** an opaque 32-byte random string in the `qz_refresh` cookie (`HttpOnly; Secure; SameSite=Strict; Path=/api/auth`). Only its SHA-256 is stored, in the existing `sessions` table, with a 30-day lifetime. `POST /api/auth/refresh` **rotates** it: the old row is deleted (zero rows deleted ⇒ 401, which also covers replay and concurrent use) and a new pair is issued.
- **Request context:** the HTTP middleware verifies the Bearer token and puts an `app.Principal{UserID}` in `context.Context`. A missing or invalid token leaves the request anonymous; `requireUser` answers 401 with `WWW-Authenticate: Bearer error="invalid_token"` so the client knows to refresh. On the client, `AuthProvider` exposes the user through React context, and the API client refreshes once on 401 and retries. Concurrent 401s share one refresh.
- **CSRF:** only `/api/auth/*` mutations still require `X-Requested-With`, because those are the only routes that read or set the cookie. Bearer-authenticated routes need no CSRF guard; browsers never attach the header on their own.
- **OAuth:** the callback sets the refresh cookie and redirects to `/`. The SPA then calls `/refresh`, so no token ever appears in a URL.

## Consequences
- Logout revokes the refresh token immediately, but an already-issued access token stays valid until it expires (at most 15 minutes). That is acceptable for this app; a deny-list would bring back the per-request lookup.
- Rotating `JWT_SECRET` logs everyone out of their access tokens; they recover silently through refresh.
- A reload costs one `/refresh` round trip before the first data request.
- Two tabs refreshing at the same instant: one wins and the other gets 401 and is sent to the login page. That is rare, because each tab refreshes about 60 seconds before expiry, staggered by when it loaded.
