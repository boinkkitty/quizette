# 3. Uploaded documents are never persisted

Status: accepted (2026-11-20)

## Decision
PDF bytes exist only in memory: the HTTP adapter streams multipart parts into memory (no `ParseMultipartForm`, which spills to temp files), the bytes travel in `domain.Document` to the in-process queue and the LLM call, then are garbage-collected. Only `doc_filename`, `doc_size`, `doc_sha256` are stored on `processing_jobs`. There is no repository for Document and no `bytea` column (asserted by an integration test). Logs never include document content (`Document.LogValue` redacts).

## Consequences
- A job in flight when the server restarts cannot be resumed; on boot such jobs are marked failed ("please re-upload").
- A durable queue would require temporarily storing documents — revisit this ADR before adopting one.
- Regenerating a chapter's questions requires re-uploading.
- With Gemini's `pdf-native` strategy the bytes are sent inline (not via the Files API, which stores files for 48h).
