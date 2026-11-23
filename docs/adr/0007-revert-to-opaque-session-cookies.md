# 7. Revert to opaque session cookies

Status: accepted (2026-11-22). Supersedes [ADR 0006](0006-jwt-access-and-refresh-tokens.md); restores the session/CSRF design of [ADR 0004](0004-same-origin-worker-proxy.md).

## Context
ADR 0006 replaced the opaque session cookie with a 15-minute JWT access token (sent as `Authorization: Bearer`) and a rotating refresh cookie. The project owner decided to return to the original design. The trade-offs are compared in [`docs/research/auth-opaque-session-vs-jwt.md`](../research/auth-opaque-session-vs-jwt.md). Its "when each is the better fit" section notes that Quizette today is a single browser app on a single backend with a cheap Postgres lookup, which is the case opaque sessions suit best.

## Decision
- On login, signup and OAuth callback the server creates a `Session` with a 32-byte random token, stores only its SHA-256 in `sessions`, and sets `qz_session` (`HttpOnly; Secure; SameSite=Lax; Path=/`, 30 days).
- The HTTP middleware resolves the cookie with `IdentityService.Authenticate` (session row + user) and puts the `domain.User` in the request context. An invalid or expired cookie is cleared and the request continues anonymously; `requireUser` answers 401.
- Every mutating `/api/*` request must carry `X-Requested-With` (CSRF guard), because the cookie rides on every request.
- Logout deletes the session row, so it takes effect immediately for every copy of the cookie.
- Removed: `adapters/jwt`, the `AccessTokens` port, `POST /api/auth/refresh`, `JWT_SECRET` / `ACCESS_TOKEN_TTL`, and the frontend token store, refresh logic and `AuthProvider`.
- Kept from the ADR 0006 period, since it isn't JWT-specific: when a page load gets a 401 (session expired or ended elsewhere), the SPA clears the cached user and sends the user to `/login?redirect=<page>`.

## Consequences
- Two small DB reads per authenticated request (session + user). This is fine at this scale; an in-process cache of session → user is the documented middle ground if it ever matters.
- Instant revocation and "log out everywhere" (delete the user's session rows).
- No signing secret to manage.
- Non-browser clients or extra backend services would need cookie handling or a call to the auth service. Revisit with ADR 0006 and the research note if that changes.
