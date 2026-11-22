# Auth styles: opaque session cookie vs JWT Bearer + refresh cookie

Status: **research note**, not a decision. Style A is implemented again as of 2026-11-22 ([ADR 0007](../adr/0007-revert-to-opaque-session-cookies.md)). Style B is recorded in [ADR 0006](../adr/0006-jwt-access-and-refresh-tokens.md), now superseded.
Written 2026-11-21 to compare both styles before committing to one.

Neither style is in git yet (the repo's only commit is `setup`). Style A's key code is shown below. Style B was removed in the working tree on 2026-11-22.

---

## Style A: opaque session token in an HttpOnly cookie (original, current)

### How it works
1. On login or signup the server generates 32 random bytes and base64url-encodes them. That string is the **session token**.
2. It stores `sha256(token)` in `sessions(token_hash PK, user_id, expires_at, created_at)` with a 30-day TTL.
3. It sets the cookie `qz_session=<token>` with `HttpOnly`, `Secure` (prod), `SameSite=Lax` and `Path=/`.
4. On every request the middleware reads the cookie, hashes it, looks up the session row and loads the user. It puts the `domain.User` in the context.
5. CSRF: every non-GET `/api/*` request must carry `X-Requested-With`. Browsers can't add a custom header cross-site without CORS, and CORS is never enabled.
6. Logout deletes the row. An hourly goroutine purges expired rows.
7. The frontend does nothing special. It sends `fetch(..., { credentials: 'same-origin' })`, and `/api/auth/me` tells it who is logged in.

### Key code (as it was)

`backend/internal/app/identity.go`
```go
const SessionTTL = 30 * 24 * time.Hour

type NewSessionToken struct {
	Token     logctx.Secret
	ExpiresAt time.Time
}

func (s *IdentityService) createSession(ctx context.Context, userID domain.ID) (NewSessionToken, error) {
	token, err := randomToken(32)
	if err != nil {
		return NewSessionToken{}, err
	}
	now := s.Clock.Now()
	sess := Session{TokenHash: hashToken(token), UserID: userID, CreatedAt: now, ExpiresAt: now.Add(SessionTTL)}
	if err := s.Repos.Sessions.Create(ctx, sess); err != nil {
		return NewSessionToken{}, err
	}
	return NewSessionToken{Token: logctx.Secret(token), ExpiresAt: sess.ExpiresAt}, nil
}

// Authenticate resolves a session token to its user (one DB read for the session, one for the user).
func (s *IdentityService) Authenticate(ctx context.Context, token string) (domain.User, error) {
	if token == "" {
		return domain.User{}, domain.ErrUnauthorized
	}
	sess, err := s.Repos.Sessions.Get(ctx, hashToken(token))
	if err != nil {
		if isNotFound(err) {
			return domain.User{}, domain.ErrUnauthorized
		}
		return domain.User{}, err
	}
	if !s.Clock.Now().Before(sess.ExpiresAt) {
		_ = s.Repos.Sessions.Delete(ctx, sess.TokenHash)
		return domain.User{}, domain.ErrUnauthorized
	}
	u, err := s.Repos.Users.GetByID(ctx, sess.UserID)
	if isNotFound(err) {
		return domain.User{}, domain.ErrUnauthorized
	}
	return u, err
}

func (s *IdentityService) LogOut(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.Repos.Sessions.Delete(ctx, hashToken(token))
}
```

`backend/internal/adapters/http/auth.go`
```go
const sessionCookie = "qz_session"

func (s *Server) setSessionCookie(w http.ResponseWriter, tok app.NewSessionToken) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: string(tok.Token), Path: "/", Expires: tok.ExpiresAt,
		HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
}
// signup/login: s.setSessionCookie(w, tok); writeJSON(w, status, toUserDTO(u))
// OAuth callback: s.setSessionCookie(w, tok); redirect to PublicURL + "/"
```

`backend/internal/adapters/http/middleware.go`
```go
const userKey ctxKey = iota
const CSRFHeader = "X-Requested-With"

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		u, err := s.cfg.Identity.Authenticate(r.Context(), c.Value)
		if err != nil {
			s.clearSessionCookie(w)
			next.ServeHTTP(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), userKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Applied to the whole /api router: every non-GET needs X-Requested-With.
func (s *Server) csrfGuard(next http.Handler) http.Handler { /* 403 if header missing on mutating methods */ }
```

`frontend/src/api/client.ts`: set `X-Requested-With: quizette` on every non-GET request, and send `credentials: 'same-origin'`. No token handling at all.

### Restoring A from B
Roughly a revert of the B-specific parts:
- Remove `adapters/jwt` and the `AccessTokens` port.
- Change `Refresh` back to `Authenticate`.
- Rename the cookie back to `qz_session` with `Path=/` and `SameSite=Lax`.
- Re-apply `csrfGuard` to all of `/api`.
- Remove `src/auth/tokenStore.ts`, `session.ts` and the retry logic in `client.ts`.

The `sessions` table is identical in both styles.

---

## Style B: JWT access token (Bearer) + rotating refresh cookie (superseded)

Full rationale: [ADR 0006](../adr/0006-jwt-access-and-refresh-tokens.md).

### How it works
1. Login, signup and refresh each return `{ user, accessToken, expiresAt }` and set the `qz_refresh` cookie (`HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/api/auth`).
2. **Access token:** an HS256 JWT. Claims are `sub`, `iss=quizette`, `aud=quizette-api`, `iat` and `exp` (15 min). It is signed with `JWT_SECRET`. The client keeps it **in memory only** and sends `Authorization: Bearer <jwt>`.
3. **Refresh token:** an opaque random string. Its SHA-256 lives in the same `sessions` table. `POST /api/auth/refresh` **rotates** it: delete the old row (0 rows deleted means 401), then issue a new pair.
4. The middleware verifies the signature and claims with **no DB access** and puts `app.Principal{UserID}` in the context. A missing or invalid token leaves the request anonymous; `requireUser` returns 401 with `WWW-Authenticate: Bearer error="invalid_token"`.
5. The client:
   - calls `/refresh` on page load;
   - schedules a proactive refresh about 60 s before expiry;
   - on a 401 it refreshes once and retries, with concurrent 401s sharing a single refresh;
   - on refresh failure it redirects to `/login`.
6. CSRF: only `/api/auth/*` POSTs need `X-Requested-With`, because those are the only routes that touch the cookie.

### Key files
- Backend:
  - `backend/internal/adapters/jwt/jwt.go`
  - `backend/internal/app/identity.go` (`Refresh`, `Verify`, `issueTokens`)
  - `backend/internal/adapters/http/{auth,middleware}.go`
- Frontend:
  - `frontend/src/auth/{tokenStore,session,context}.ts`
  - `frontend/src/auth/AuthProvider.tsx`
  - `frontend/src/api/client.ts`

---

## Comparison

| | A: opaque session cookie | B: JWT Bearer + refresh cookie |
|---|---|---|
| DB reads per authenticated request | 2 (session + user) | 0 (refresh only: every ~15 min, plus on page load) |
| Logout / revoke takes effect | Immediately | Refresh token immediately; access token **up to 15 min later** |
| "Log out everywhere" / ban user | Delete rows → instant | Delete rows → effective within 15 min (or add a deny-list / token version check, which brings the DB lookup back) |
| XSS impact | Attacker can make requests while the page is open, but can't read the cookie | Same, plus they can read the in-memory access token (valid ≤15 min). The refresh token is never readable. |
| CSRF exposure | Cookie on every request → needs the CSRF guard everywhere | Only `/api/auth/*` is cookie-authenticated; data routes are CSRF-immune by construction |
| Frontend complexity | Almost none | Token store, single-flight refresh, retry-on-401, proactive timer, boot refresh |
| Backend complexity | Small | Signing key management, claim validation, rotation, extra endpoint |
| Secrets to manage | None beyond DB | `JWT_SECRET` (rotating it forces everyone to refresh) |
| Page load | One `/me` call | One `/refresh` call before data requests (same count) |
| Non-browser clients (CLI, mobile, other services) | Awkward; they'd need cookie handling | Natural: `Authorization: Bearer` is the standard |
| Splitting into multiple services later | Each needs DB access to the sessions table, or calls the auth service | Any service with the key (or a public key, if switched to RS256/EdDSA) can verify locally |
| Multi-tab edge cases | None | Two tabs refreshing in the same instant: one wins, the other is logged out (rare) |
| Behind the Cloudflare Worker proxy | Works (same-origin cookie) | Works (same-origin refresh cookie + forwarded header) |
| Failure modes to get right | Cookie flags, CSRF guard | Algorithm pinning (`alg: none` / RS↔HS confusion), `aud`/`iss`/`exp` checks, rotation race, key strength |

### When each is the better fit
- **Choose A** when it's a single browser app on a single backend with a cheap DB lookup (Postgres, and Neon's pooler is fine at this scale), and you want instant revocation and the least code. This describes Quizette today.
- **Choose B** when you expect non-browser clients, multiple backend services, or enough request volume that per-request session lookups matter. It also fits if you want the API contract to be the industry-standard Bearer scheme.

### Middle-ground options worth researching
1. **A plus a short in-process cache** of session → user (e.g. 30–60 s). This removes most DB reads but keeps near-instant revocation.
2. **B plus a `token_version` column** on users, checked on each request. This gives instant "log out everywhere" but brings back one DB read.
3. **B with asymmetric keys (EdDSA/RS256) + JWKS**, only if other services need to verify tokens without holding the signing secret.
4. **B-in-a-cookie** (the JWT stored in an HttpOnly cookie instead of memory). This keeps the stateless verification but brings back CSRF on all routes and loses the Bearer contract. It's usually the worst of both.

### Further reading
- OWASP Session Management Cheat Sheet
- OWASP JSON Web Token Cheat Sheet (Java-flavoured, but the pitfalls are generic)
- RFC 6749 §1.5 and §10.4 (refresh tokens), RFC 6750 (Bearer usage), RFC 8725 (JWT Best Current Practices)
- IETF draft "OAuth 2.0 for Browser-Based Applications" (token storage in SPAs, refresh token rotation)
