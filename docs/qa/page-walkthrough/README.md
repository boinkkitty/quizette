# QA walk-through: every page, desktop and mobile

Done on 2026-11-25 for the ticket "QA walk-through of every page on desktop and mobile" (#6). The app ran locally at `main` (c162eb1) with `LLM_PROVIDER=stub`. The browser was ego-browser at 1280×800 (`d-*.png`) and 390×844 with mobile emulation (`m-*.png`).

Seed data: one account with three topics.

- **Biology:** Cell structure (ready, 15 questions), Photosynthesis (ready), Genetics basics (failed).
- **Employee handbook:** Leave policy and Expenses and travel, both ready.
- **Spanish recipes:** only a failed chapter.

It also had one submitted attempt and four abandoned in-progress attempts. A second, empty account was used for the empty states.

This file records facts and suggestions only. Sitemap decisions belong to the sitemap ticket.

## Routes

| Route | Purpose | Links in | Links out (page body) | Shots |
|---|---|---|---|---|
| `/login` | Log in with email or OAuth | Auth guard redirect, Sign out, `/signup` | `/signup`, Google, GitHub, then the redirect target | [d](d-login.png) |
| `/signup` | Create an account | `/login` | `/login` | [d](d-signup.png) |
| `/` "Your topics" (sidebar label: Dashboard) | Grid of topics with chapter counts | Sidebar Dashboard, brand logo, "← All topics", "Back to dashboard" on 404 and not-found, after deleting a topic | `/upload` (header button), `/topics/:id` | [d](d-dashboard.png) [m](m-dashboard.png) [empty](m-empty-dashboard.png) |
| `/topics/:id` | A topic's chapters with status, plus Delete topic and Add chapter | `/` only, and the chapter page's back link | `/`, `/upload?topicId=`, `/chapters/:id` | [d](d-topic.png) [m](m-topic.png) [failed only](m-topicFailedOnly.png) [delete](m-deleteDialog.png) |
| `/chapters/:id` (ready) | Chapter hub: start a quiz with a count, submitted attempts, inline question bank toggle, delete | Topic page; `/quizzes/:topicId` (both the title **and** "Start quiz"); History (chapter title); Results back link; upload "Go to chapter" | `/topics/:id`, start → `/attempts/:id`, Review → `/attempts/:id/results`. **No link to `/chapters/:id/questions`.** | [d](d-chapterReady.png) [m](m-chapterReady.png) |
| `/chapters/:id` (failed) | Failure reason and a re-upload link | Same as ready | `/topics/:id`, `/upload?topicId=` | [d](d-chapterFailed.png) [m](m-chapterFailed.png) |
| `/chapters/:id/questions` | Study walkthrough: one question per screen, "Show answer" | Only "View questions" on `/quizzes/:topicId` | Back link **and** Done both go to `/quizzes/:topicId`, not to the chapter | [d](d-chapterQuestions.png) [m](m-chapterQuestions.png) |
| `/quizzes` | Topics that have at least one ready chapter | Sidebar, History empty state | `/quizzes/:topicId` | [d](d-quizzes.png) [m](m-quizzes.png) [empty](m-empty-quizzes.png) |
| `/quizzes/:topicId` | A topic's ready chapters with "View questions" and "Start quiz" | `/quizzes`, the questions page | `/quizzes`, `/chapters/:id` (title), `/chapters/:id` ("Start quiz": does not start a quiz), `/chapters/:id/questions` | [d](d-quizzesTopic.png) [m](m-quizzesTopic.png) [none ready](m-quizzesTopicFailedOnly.png) |
| `/attempts/:id` | Quiz runner: one question per screen, then Submit | Chapter "Start quiz", History "Resume" | **None in the page body.** Submit goes to results. | [d](d-attemptInProgress.png) [m](m-attemptInProgress.png) [last q](d-quizLastQuestion.png) |
| `/attempts/:id/results` | Score and per-question review | After submit, History "Review", chapter attempt "Review" | Only "← chapter" at the top | [d](d-results.png) [m](m-results.png) [bottom](d-resultsBottom.png) |
| `/history` | Every attempt, newest first | Sidebar | `/chapters/:id`, `/attempts/:id` (Resume), `/attempts/:id/results` (Review). No topic link. | [d](d-history.png) [m](m-history.png) [empty](m-empty-history.png) |
| `/upload` | Upload form, then generation progress, then a ready link | Sidebar; Dashboard button; topic "Add chapter"; failed-chapter re-upload; empty states | Once ready: "Go to chapter →", "Upload another PDF" | [d](d-upload.png) [m](m-upload.png) [progress](d-uploadProgress.png) [done](d-uploadDone.png) |
| Not found (route or resource) | Error message | Any bad URL | `/` | [d](d-notFoundRoute.png) [m](m-badChapter.png) |

The mobile drawer works: focus is trapped, Escape closes it, and it closes when you navigate ([m](m-drawer.png)).

## Dead ends

- **Quiz runner** (`/attempts/:id`): no back link or exit. The only ways out are the sidebar or browser Back. It is the only page without a header back link.
- **Results** (`/attempts/:id/results`): the only exit is the back link at the top. After a long review (15 questions is several phone screens), nothing at the bottom offers "retake", "next chapter" or "History" ([bottom](d-resultsBottom.png)).

## Duplicate paths

- **Two parallel topic → chapter trees** with different contents:
  - `/` → `/topics/:id` includes failed chapters.
  - `/quizzes` → `/quizzes/:topicId` shows only ready chapters.

  Both end on the same chapter page.
- **Two question-bank viewers:** "Review question bank" expands a list with answers inline on the chapter page, and `/chapters/:id/questions` is a separate walkthrough reachable only from Quizzes.
- **On `/quizzes/:topicId`:** the chapter title and "Start quiz" go to the same chapter page.
- **Upload calls to action:** the sidebar "Upload" and the Dashboard header "Upload PDF" sit side by side on desktop.
- **The "← Biology" label** goes to `/topics/:id` on the chapter page but to `/quizzes/:topicId` on the questions page. Same text, different destinations.

## Per-page issues

**Flow and state**

- **Chapter page hides in-progress attempts.** Photosynthesis has two in-progress attempts. Its page says "No attempts yet" and offers no Resume. Pressing Start quiz creates another attempt. History builds up abandoned "In progress" rows (four of five rows in the seed), and there is no way to discard them.
- **Starting a quiz from Quizzes takes 3 clicks** (topic → "Start quiz" → "Start quiz"). The first "Start quiz" is a link to the chapter page, not a start.
- **Upload success** offers only "Go to chapter →", not "Start quiz". The success state is a small banner, and the page loses its vertical centring ([done](d-uploadDone.png)).
- **No route-level pending UI.** The code has no `pendingComponent` or `defaultPendingComponent`. On a slow API, such as a Render cold start, a click does nothing visible until the loader resolves. This comes from reading the code; locally it was not observable.

**Data shown**

- **Pluralisation:** "1 attempts" appears on `/topics/:id` and `/quizzes/:topicId`.
- **Counts disagree:** the Dashboard says Biology has "3 chapters" (counting the failed one), while Quizzes says "2 chapters".
- **Failed rows:** a failed chapter's row on `/topics/:id` has a blank subtitle, with no reason and no next step.
- **Hidden topics:** a topic with only failed or pending chapters (Spanish recipes) does not appear on `/quizzes`.

**Hierarchy**

- **Delete buttons:** "Delete topic" and "Delete chapter" are as prominent as the primary action. On a failed chapter, Delete is the only button.
- **Quiz runner header:** the header ("Cell structure / 10-question quiz") is left-aligned while the runner is a centred column (desktop).

**Mobile (390 px)**

- **History overflows.** Its sr-only "Actions" `<span>` is absolutely positioned with no positioned ancestor, so it escapes the table's `overflow-x-auto` wrapper. The document becomes 570 px wide and phones zoom the page out. Score and Resume/Review are also off-screen behind the table's horizontal scroll ([m](m-history.png)).
- **Chapter page overflows by 14 px.** The grid column grows to fit the attempt table, and the date wraps onto 4 lines ([m](m-chapterReady.png)).
- **Tap targets under 44 px:**
  - Text links are 18–21 px tall: every "←" back link, the History chapter, Resume and Review links, and the empty-state links.
  - Buttons are 32–36 px: Start quiz, View questions, Show answer, Previous/Next, Delete, Add chapter, and the Dashboard's Upload PDF.
  - Quiz answer options are full-row labels of about 46 px, which is fine.
- **Topic header:** Delete topic and Add chapter sit next to the title at equal weight.
- **Quiz controls:** Previous/Next sit below the card, so their position moves with each question's length and is not anchored for the thumb. This feeds the "mobile-specific flow" fog.

**Copy**

- No copy assumes lectures or students. The sign-up subtitle reads "Turn any PDF into a practice quiz."
- "practise" (upload hint, Quizzes subtitle) and "practice" (sign-up) are both used.
- The `/` subtitle explains the data model ("Each topic holds chapters; each chapter is a question bank generated from one PDF.") instead of helping the user act.

**Empty and error states:** Dashboard, Quizzes and History each give one clear next step. The failed chapter shows the reason and a re-upload link to the same topic. Not-found pages link to `/`.

## Suggested improvements (ordered by impact)

1. On `/chapters/:id`, show in-progress attempts with **Resume** → `/attempts/:id`. Or make Start quiz resume the open attempt instead of creating another one.
2. On `/attempts/:id`, add a header back link "← <chapter>" → `/chapters/:id`, and a way to quit.
3. On `/attempts/:id/results`, add a bottom action bar with "Retake" (new attempt), "Back to chapter" → `/chapters/:id` and "History" → `/history`.
4. On `/quizzes/:topicId`, make "Start quiz" start an attempt directly → `/attempts/:id`, with the default count or an inline picker. Drop the duplicate title link or make it clearly "details".
5. On `/history`, fix the sr-only overflow. On mobile, render rows as cards so score and Resume/Review are visible without scrolling.
6. On `/chapters/:id`, link "View questions" → `/chapters/:id/questions`, or replace the inline bank with it. Point that page's back link and Done back to `/chapters/:id`.
7. On `/upload` success, add "Start quiz" → `/chapters/:id` (or straight into an attempt) next to "Go to chapter".
8. On `/history`, let users discard stale in-progress attempts, or hide them.
9. On `/topics/:id`, give failed rows a subtitle "Generation failed: re-upload", linking to `/upload?topicId=`.
10. Fix "1 attempts". Make chapter counts agree between `/` and `/quizzes`, e.g. "3 chapters · 1 failed".
11. On `/chapters/:id`, add `min-w-0` to the grid items or put `overflow-x-auto` around the attempt table to stop the mobile overflow.
12. Raise mobile tap targets to 44 px or more (back links, table links, card buttons). On mobile, move Delete topic and Delete chapter into an overflow menu.
13. Add a router-level pending indicator (a top progress bar or `defaultPendingComponent`) for cold starts.
14. On `/`, drop the "Upload PDF" header button, which duplicates the sidebar (moot once the Dashboard is redesigned).
15. Copy: settle on "practise" or "practice", and replace the data-model subtitle on `/`.

Structural questions are left to the sitemap ticket: the two topic trees, where the question walkthrough lives, and what `/` becomes.
