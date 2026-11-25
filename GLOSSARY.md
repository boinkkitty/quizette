# Glossary

Use these terms in code, issues and docs. "Avoid" lists synonyms that should not appear in code.

| Term | Meaning | Avoid |
|---|---|---|
| **Topic** | A user-named group of chapters (e.g. "Biology"). | Subject, course, folder |
| **Chapter** | The unit created by one PDF upload; belongs to one topic and owns one question set. Status: pending → ready / failed. | Lesson, deck |
| **Deleted chapter / Deleted topic** | Deleted by the user and hidden everywhere, but kept because an attempt refers to it, so past attempts stay as they were. Cannot be restored. | Archived, trashed |
| **Document** | The uploaded PDF, held in memory only while processing. Never persisted; only its metadata (filename, size, SHA-256) is stored on the job. | File, upload (as a noun in code) |
| **ProcessingJob** | One attempt to turn a document into a question set. Status: queued → running → succeeded / failed. Carries generation metrics (tokens, timings, strategy). | Task, generation |
| **QuestionSpec** | What to generate: count (5–20, multiple of 5) and type mix. | |
| **QuestionSet** | A chapter's immutable bank of questions. "Question bank" is fine in UI copy. | Quiz (for the bank) |
| **Question** | One item in a set: MCQ, True/False or MRQ, with options, correct indices and an explanation. | |
| **MCQ** | Multiple choice: 4 options, exactly 1 correct. | |
| **True/False** | Options exactly `True`, `False`; 1 correct. Code value `true_false`. | |
| **MRQ** | Multiple-response ("select all that apply"): 3–5 options, at least 1 correct (all may be correct). Partial credit. | Multi-select, checkbox question |
| **Score** | Points in [0, 1] for one answer. MRQ: `max(0, (right − wrong) / totalCorrect)`. | Grade (as a noun) |
| **Attempt** | One finished practice run over N questions (N multiple of 5 when it starts), recorded with answers and score when the quiz is submitted. Has an attempt scope. | ChapterAttempt, quiz attempt, session |
| **Unfinished quiz** | A quiz being taken: its questions and answers so far, held only in its browser tab. Becomes an Attempt when submitted; lost if quit or the tab closes. | In-progress attempt, draft |
| **Attempt scope** | What an attempt draws from: one `chapter`, or a whole `topic` (its ready chapters). | |
| **Quiz** | UI word for an Attempt; "topic quiz" and "chapter quiz" name its scope. Not a separate concept in code. | |
| **Generation strategy** | How the document is fed to the LLM: `pdf-native` (PDF bytes) or `text-extract` (text from ledongthuc/pdf). | Mode |
| **QuestionGenerator** | The port every LLM adapter implements. | LLM client (in app/domain) |
| **Session** | A login: an opaque random token in the `qz_session` HttpOnly cookie, stored only as its SHA-256 in `sessions` (30-day TTL). Deleted on logout. | Access token, auth token |
