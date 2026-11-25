# 10. Scoped attempts and soft-deleted chapters and topics

Status: accepted (2026-11-25).

## Context
A `ChapterAttempt` belongs to exactly one chapter (`chapter_attempts.chapter_id`). Users now want to quiz themselves on a whole topic. Deleting a chapter or topic cascades through its attempts and answers, so a topic quiz that drew from a later-deleted chapter would silently lose answers and its score would stop matching what was asked.

## Decision
- **One `Attempt` aggregate with an attempt scope.** `ChapterAttempt` becomes `Attempt` with `scope ∈ {topic, chapter}` and exactly one of `topic_id` / `chapter_id` set (CHECK constraint). An answer's chapter is derived through question → question set → chapter. Rejected: a topic attempt as a parent of one ChapterAttempt per chapter (one quiz becomes N aggregates to stitch together on submit, resume and history), and a separate `TopicAttempt` aggregate (duplicates grading and history).
- **Selection is random, spread across chapters.** A topic attempt splits N across the topic's ready chapters in proportion to each chapter's question count (largest-remainder rounding), then draws at random within each chapter. Chapter attempts also draw at random. The never-attempted → previously-wrong → rest priority and its per-question history query are removed.
- **Count rule.** N is a multiple of 5, at least 5, and at most min(pool, 50), where the pool is the questions of the scope's ready chapters. An attempt cannot start with no ready chapter. The multiple-of-5 rule applies when an attempt starts only.
- **Chapter stats** (attempt count, best score) count chapter-scoped attempts only.
- **Delete keeps what attempts refer to.** Deleting a chapter that any attempt (either scope) references marks it deleted instead of removing it. Deleting a topic does the same to the topic and its chapters when any attempt references the topic or one of its chapters. Otherwise, the row is removed as today. Deleted chapters and topics are hidden everywhere (browser, pools, counts, upload targets) and cannot be restored, but past attempts keep their frozen score, and reviews show their questions labelled as coming from deleted content. (Unfinished quizzes are not attempts; see ADR 0011.) A deleted topic's name is free for a new topic. Rejected: shrinking attempts and recomputing their score (History rewrites itself), and deleting affected attempts (loses history).
- New chapters do not change past topic attempts: `question_ids` stays a snapshot.

## Consequences
- Schema change: `chapter_attempts` → `attempts` with scope columns, `deleted_at` on `chapters` and `topics`, the `(owner_id, name)` topic uniqueness becomes a partial index over non-deleted topics, and every library query filters out deleted rows.
- Delete no longer guarantees data is gone. Questions derived from a user's documents stay stored while an attempt refers to them, even after the user deletes the chapter or topic. Only removing the user row (no user-facing account deletion exists today) cascades them away.
- Without history-driven selection, a chapter quiz may repeat questions the user just answered correctly. Accepted for simplicity.
