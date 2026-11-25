# 11. Attempts are created on submit; unfinished quizzes live in the tab

Status: accepted (2026-11-25).

## Context
Starting a quiz creates an `in_progress` attempt on the server, while answers are kept only in the tab's `sessionStorage`. The server cannot tell a quiz being taken from an abandoned one: leaving half-way leaves an "In progress" row that only History can resume, pressing Start again creates a second attempt, and History fills with rows nothing can discard. Soft-deleted content (ADR 0010) also had to stay submittable for these rows.

## Decision
- **No server state while a quiz runs.** Start calls a stateless draw endpoint that applies the scope and count rules (ADR 0010) and returns the drawn questions without correct answers. The tab holds the draw and the answers in one `sessionStorage` slot; starting another quiz overwrites the slot.
- **Submit creates the attempt.** The client sends the scope, the drawn question ids and the responses. The server checks the ids belong to the scope's ready, non-deleted chapters and that the count is valid, grades, and stores an `Attempt` that is already submitted. `AttemptStatus` and `in_progress` are removed.
- **The tab is the quiz's lifetime.** Quit (confirmed) clears the slot; closing the tab or logging out loses it. There is no Resume UI and no expiry. A submit that names deleted questions is rejected and the quiz can only be discarded.
- Rejected: keeping the server attempt and adding Resume, Discard and expiry on top (patches every symptom, keeps the abandoned-row problem), and saving each answer to the server for cross-device resume (more API and state for little value in a self-practice app).

## Consequences
- History, chapter stats and soft-delete only ever see submitted attempts; there is nothing to abandon or clean up.
- No cross-device or cross-tab resume, and closing the tab loses the quiz.
- The client says which questions it was asked. The server still validates scope membership and grades, so the only possible cheating is against yourself.
