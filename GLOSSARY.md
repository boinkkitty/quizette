# Glossary

Use these terms in code, issues and docs. "Avoid" lists synonyms that should not appear in code.

| Term | Meaning | Avoid |
|---|---|---|
| **Topic** | A user-named group of chapters (e.g. "Biology"). | Subject, course, folder |
| **Chapter** | The unit created by one PDF upload; belongs to one topic and owns one question set. Status: pending → ready / failed. | Lesson, deck |
| **Document** | The uploaded PDF, held in memory only while processing. Never persisted; only its metadata (filename, size, SHA-256) is stored on the job. | File, upload (as a noun in code) |
| **ProcessingJob** | One attempt to turn a document into a question set. Status: queued → running → succeeded / failed. Carries generation metrics (tokens, timings, strategy). | Task, generation |
| **QuestionSpec** | What to generate: count (5–30, multiple of 5) and type mix. | |
| **QuestionSet** | A chapter's immutable bank of questions. "Question bank" is fine in UI copy. | Quiz (for the bank) |
| **Question** | One item in a set: MCQ, True/False or MRQ, with options, correct indices and an explanation. | |
| **MCQ** | Multiple choice: 4 options, exactly 1 correct. | |
| **True/False** | Options exactly `True`, `False`; 1 correct. Code value `true_false`. | |
| **MRQ** | Multiple-response ("select all that apply"): 3–5 options, 2..(options−1) correct. Partial credit. | Multi-select, checkbox question |
| **Score** | Points in [0, 1] for one answer. MRQ: `max(0, (right − wrong) / totalCorrect)`. | Grade (as a noun) |
| **ChapterAttempt** | One practice run over N questions drawn from a chapter's set (N multiple of 5). Recorded with answers and score. | Quiz attempt, session |
| **Quiz** | UI word for a ChapterAttempt. Not a separate concept in code. | |
| **Generation strategy** | How the document is fed to the LLM: `pdf-native` (PDF bytes) or `text-extract` (text from ledongthuc/pdf). | Mode |
| **QuestionGenerator** | The port every LLM adapter implements. | LLM client (in app/domain) |
| **Session** | A login: an opaque random token in the `qz_session` HttpOnly cookie, stored only as its SHA-256 in `sessions` (30-day TTL). Deleted on logout. | Access token, auth token |
