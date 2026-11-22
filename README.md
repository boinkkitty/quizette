# Quizette

Upload a PDF, get a reusable bank of LLM-generated questions (multiple choice, true/false, select-all-that-apply), and practise with quizzes. PDFs are parsed in memory and **never stored**.

- **Frontend** — React 19 + TypeScript (strict) + TanStack Router/Query + Tailwind v4 + shadcn/ui (Radix; blue & white theme in `src/styles.css`), zod-validated API contract · `frontend/`
- **Edge** — Cloudflare Worker: serves the SPA, proxies `/api/*` to the API (same-origin session cookie) · `worker/`
- **Backend** — Go 1.27, chi, pgx + sqlc, goose, `log/slog`; hexagonal / DDD · `backend/`
- **Database** — PostgreSQL (Neon in production)
- **LLM** — Gemini 3.8 Flash behind a swappable `QuestionGenerator` port; schema-constrained JSON output

Read [docs/architecture.md](docs/architecture.md), [GLOSSARY.md](GLOSSARY.md) and [docs/adr/](docs/adr/).

Still to do by hand (API keys, OAuth apps, deployment): see [SETUP.md](SETUP.md).

## Local development

Prerequisites: Go (any recent version; `go.mod` pins toolchain 1.27.4 and downloads it), Node 22+, pnpm 11 (`corepack enable`), Docker.

Settings live in git-ignored `.env.local` files. Each folder has a `.env.example` template listing every variable:

```sh
cp backend/.env.example  backend/.env.local     # PORT, PUBLIC_URL, DATABASE_URL, LLM, OAuth…
cp frontend/.env.example frontend/.env.local    # DEV_PORT, API_PROXY_TARGET
cp e2e/.env.example      e2e/.env.local         # only for Playwright
```

Keep the ports consistent: the frontend's `API_PROXY_TARGET` must point at the backend's `PORT`, and the backend's `PUBLIC_URL` must use the frontend's `DEV_PORT`.

```sh
docker compose up -d db                      # dev Postgres (credentials in docker-compose.yml)

cd backend && make dev                       # loads .env.local; LLM_PROVIDER=stub needs no key
cd frontend && pnpm install && pnpm dev      # loads .env.local; proxies /api to the API
```

To use Gemini, set `LLM_PROVIDER=gemini` and `GEMINI_API_KEY` in `backend/.env.local`. Migrations run automatically when the API starts.

## Tests & checks

| | Command |
|---|---|
| Backend unit tests | `cd backend && make test` |
| Backend + Postgres integration (Docker) | `make test-integration` |
| Backend lint (golangci-lint v2, depguard layer rules) | `make lint` |
| Frontend typecheck + lint + Vitest | `cd frontend && pnpm check` |
| E2E (Playwright; needs `docker compose up -d db` and `e2e/.env.local`) | `cd e2e && pnpm install && pnpm exec playwright install chromium && pnpm test` |
| Generation strategy benchmark (needs `GEMINI_API_KEY` in `backend/.env.local`) | `cd backend && make bench` |

## Deploy

1. **Neon** — create a project; copy the *pooled* connection string.
2. **Render** — new Blueprint from `render.yaml` (Docker, `backend/`). Set `DATABASE_URL`, `GEMINI_API_KEY`, `PUBLIC_URL` (the Worker URL), and optional OAuth secrets.
3. **Cloudflare** — set `API_ORIGIN` in `worker/wrangler.jsonc` to the Render URL, then `cd worker && pnpm install && pnpm run deploy`.
4. **OAuth** (optional) — register redirect URIs `https://<worker-host>/api/auth/google/callback` and `.../github/callback`.

Render's free tier sleeps after 15 idle minutes; the UI shows a "waking up the server" banner during the ~30–60s cold start.

See `backend/.env.example` for every backend setting.
