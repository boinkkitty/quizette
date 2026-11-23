# 5. Generation strategy: PDF-native vs. extracted text

Status: **proposed — awaiting benchmark results**

## Context
Two ways to feed a document to Gemini, both implemented behind `QuestionGenerator`:
1. `pdf-native` — PDF bytes sent inline (`application/pdf`); Gemini handles layout, tables, figures and scans.
2. `text-extract` — text extracted locally with `github.com/ledongthuc/pdf`, sent as plain text.

We want to compare latency (extract / LLM / total), token usage (input / output / thinking) and validation pass rate.

## Method
```
cd backend
make bench   # GEMINI_API_KEY from backend/.env.local; = go run ./cmd/genbench -dir testdata/pdfs -runs 3 -count 10
```
Fixtures go in `backend/testdata/pdfs` (git-ignored; `make fixture` writes a small sample): short text, ~30-page, tables/diagrams, scanned. Results also accumulate in production in `processing_jobs` metric columns.

## Results
_To be filled in from `genbench` output._

## Decision
Default `GENERATION_STRATEGY=pdf-native` until results say otherwise (it is the only strategy that works on scanned PDFs).
