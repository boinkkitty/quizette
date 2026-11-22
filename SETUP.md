# Setup: what's left to do by hand

The code is done and CI passes. These steps need your accounts, keys or decisions, so they can't be automated from the repo. Work through them in order; each one unblocks the next.

## Checklist

- [ ] 1. Make the decisions below
- [ ] 2. Get a Gemini API key and run the live smoke test
- [ ] 3. Run the generation-strategy benchmark
- [ ] 4. Merge PR #1
- [ ] 5. Create the Neon database
- [ ] 6. Deploy the API on Render
- [ ] 7. Deploy the Cloudflare Worker
- [ ] 8. Create the Google and GitHub OAuth apps
- [ ] 9. Check the live site
- [ ] 10. Optional hardening

For local development, see [README.md](README.md#local-development).

---

## 1. Decisions

Settle these first; several steps depend on them.

| Decision | Options | Affects |
|---|---|---|
| **Domain** | A custom domain, or the free `*.workers.dev` (site) and `*.onrender.com` (API) | OAuth redirect URLs, `PUBLIC_URL`, `API_ORIGIN` |
| **Render plan** | Free: sleeps after 15 idle minutes, ~30–60 s cold start (the UI shows a "waking server" banner), and in-progress question generation is lost when it sleeps or restarts. Starter ($7/month): always on. | Step 6 |
| **Gemini tier** | Free: Google may use the uploaded documents to train its models, and rate limits are low. Paid: documents are not used for training. | Step 2; what you tell users |

## 2. Gemini API key and live smoke test

The app has never made a real Gemini call. The main unknown is whether Gemini accepts the JSON schema we send for structured output.

1. Create a key at [Google AI Studio](https://aistudio.google.com/apikey).
2. Put it in `backend/.env.local` and switch the provider:
   ```sh
   LLM_PROVIDER=gemini
   GEMINI_API_KEY=your-key
   ```
3. Run the live test (it's skipped unless the key is set):
   ```sh
   cd backend
   set -a; . ./.env.local; set +a
   go test ./internal/adapters/llm/gemini -run TestLiveGemini -v
   ```
4. Try it end to end: run `make dev` and `pnpm dev`, then upload a real PDF at http://localhost:5173.

If the test fails with a schema error, the schema is in `backend/internal/app/question_set.schema.json`.

## 3. Generation-strategy benchmark

This decides whether we send the PDF itself to Gemini (`pdf-native`, the current default) or text we extract first (`text-extract`). The benchmark measures time and tokens for both.

1. Add realistic PDFs to `backend/testdata/pdfs/` (they're git-ignored). See the [fixtures README](backend/testdata/pdfs/README.md) for the recommended mix: short text, ~30 pages, tables/diagrams, and a scanned one.
2. With `GEMINI_API_KEY` set in `backend/.env.local`:
   ```sh
   cd backend && make bench
   ```
   It pauses between calls to stay inside free-tier rate limits, so it takes a few minutes.
3. Copy the results table into `docs/adr/0005-generation-strategy.md`, change its status from "proposed" to "accepted", and set `GENERATION_STRATEGY` in `render.yaml` to the winner. Note that only `pdf-native` handles scanned PDFs.

## 4. Merge PR #1

Render and Cloudflare deploy from `main`, so merge https://github.com/boinkkitty/quizette/pull/1 first.

## 5. Neon database

1. Create a project at [neon.tech](https://neon.tech). Pick a region close to your Render region.
2. Copy the **pooled** connection string. Its host contains `-pooler`. It goes in `DATABASE_URL` in step 6.

The API runs migrations itself on startup, so there's nothing to run by hand.

## 6. API on Render

1. In [Render](https://dashboard.render.com): **New → Blueprint**, connect the `boinkkitty/quizette` repo. It reads `render.yaml` and creates the `quizette-api` web service (Docker, `backend/`).
2. Pick the plan you chose in step 1 (`render.yaml` says `free`).
3. Set the environment variables it asks for:

   | Variable | Value |
   |---|---|
   | `DATABASE_URL` | Neon pooled URL from step 5 |
   | `GEMINI_API_KEY` | From step 2 |
   | `PUBLIC_URL` | The site's URL from step 7, e.g. `https://quizette.<you>.workers.dev`. Put a placeholder for now and update it after step 7. |
   | `GOOGLE_*`, `GITHUB_*` | Leave empty until step 8 |

   `APP_ENV=production` and `GENERATION_STRATEGY` come from `render.yaml`. Render sets `PORT` itself. To change a tunable such as `JOB_TIMEOUT`, add it here; see `backend/.env.example` for the full list.
4. Deploy. When it's live, `https://<service>.onrender.com/readyz` should return `{"status":"ok"}`. Note this URL for step 7.

## 7. Cloudflare Worker (the site)

1. Log in once: `cd worker && pnpm install && pnpm exec wrangler login`.
2. In `worker/wrangler.jsonc`, set `vars.API_ORIGIN` to your Render URL from step 6. The current value, `https://quizette-api.onrender.com`, is a guess.
3. Deploy (this builds the frontend first):
   ```sh
   pnpm run deploy
   ```
   Wrangler prints the site URL, e.g. `https://quizette.<you>.workers.dev`.
4. Back in Render, set `PUBLIC_URL` to that URL with no trailing slash, and redeploy.
5. Commit the `wrangler.jsonc` change.

For a custom domain, add it to the Worker in the Cloudflare dashboard and use it for `PUBLIC_URL` instead.

## 8. OAuth apps (Google and GitHub)

Sign-in with email and password works without this; each provider's button appears once its client ID is set. The callback URL is always `<site URL>/api/auth/<provider>/callback`.

### Google

1. In [Google Cloud Console](https://console.cloud.google.com/apis/credentials), set up the **OAuth consent screen** (External, scopes `openid`, `email`, `profile`).
2. **Create credentials → OAuth client ID → Web application**, with these authorized redirect URIs:
   - `http://localhost:5173/api/auth/google/callback` (local dev)
   - `https://<your-site>/api/auth/google/callback` (production)
3. Copy the client ID and secret into `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`: in Render for production, and in `backend/.env.local` for local dev.
4. While the consent screen is in "Testing" mode, only test users you add can sign in. Publish it when you're ready.

### GitHub

GitHub OAuth apps allow only one callback URL, so create **two apps**: one for local dev, one for production.

1. Go to [GitHub → Settings → Developer settings → OAuth Apps → New OAuth App](https://github.com/settings/applications/new).
2. Production app: homepage `https://<your-site>`, callback `https://<your-site>/api/auth/github/callback`.
3. Dev app: homepage `http://localhost:5173`, callback `http://localhost:5173/api/auth/github/callback`.
4. Generate a client secret for each. Put the production pair in Render (`GITHUB_CLIENT_ID` / `GITHUB_CLIENT_SECRET`) and the dev pair in `backend/.env.local`.

Redeploy on Render after adding the variables.

## 9. Check the live site

On the deployed URL:

- [ ] The first visit after idle shows the "waking server" banner, then the app loads (free plan only).
- [ ] Sign up with email and password, reload the page, and you're still signed in. The session cookie works through the Worker.
- [ ] Sign in with Google and with GitHub, each landing on the dashboard.
- [ ] Upload a real PDF. It goes from Generating to Ready, and the questions make sense.
- [ ] Take a quiz, check the results and History, and try View questions.
- [ ] Sign out, and a second account can't see the first account's topics.
- [ ] In Neon, `processing_jobs` rows have tokens and timings filled in.

## 10. Optional hardening

- **Branch protection:** in GitHub repo settings, require the `backend`, `frontend` and `e2e` checks to pass before merging to `main`.
- **Corepack:** pnpm is pinned to 11 because the installed corepack (0.34) can't run pnpm 12. Upgrade corepack before moving to pnpm 12.
- **Upload limit:** there is currently no cap on uploads, so one user can spend the whole Gemini quota. If you add one, count uploads in a separate table that isn't deleted along with chapters (deleting a chapter also deletes its processing jobs). Do the count and the insert in one transaction, and check before the file is read.
- **Features deliberately left out**, if you want them later: email verification and password reset (needs an email provider such as Resend), editing questions, spaced repetition, and a durable job queue. Regenerating a chapter needs a re-upload, because PDFs are never stored.
