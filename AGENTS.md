## Agent skills

### Issue tracker

Issues and specs are tracked in GitHub Issues for `boinkkitty/quizette`. See `docs/agents/issue-tracker.md`.

### Triage labels

Uses the default five canonical triage labels. See `docs/agents/triage-labels.md`.

### Domain docs

Uses a single-context layout. See `docs/agents/domain.md`.

## Pull Requests

### Title

Use a semantic title: `type(scope): short description`.

### Description

Keep the description brief and answer:

- What changed?
- Why was it needed?
- How was it tested?

Link relevant issues. For performance changes, include before-and-after measurements.

### CI

After opening the PR, check that all CI checks pass. Investigate and fix failures caused by the PR.

## Go Code Style

- Follow [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) and the [formatting and style guidance in Go: Best Practices for Production Environments](https://peter.bourgon.org/go-in-production/#formatting-and-style).
- State assumptions that affect implementation or behavior.
- Document ownership and lifetime rules at the interface definition when callers need them, including whether buffers may be reused.
- Add a doc comment to every exported declaration, following the [Go doc comment guide](https://go.dev/doc/comment).
- Write prose comments as sentences that start with a capital letter and end with a full stop. Start declaration doc comments with the name being documented.

## Testing

- Add or update tests for relevant behavior. Avoid trivial tests written only to increase coverage.
- For bug fixes, add a test that reproduces the bug.
- For new behavior or exported API changes, add unit or end-to-end tests.
- For HTTP tests, use a real `httptest` server instead of mocking HTTP interactions.
