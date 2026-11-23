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

### Run 1 — 2026-11-23, inconclusive
`go run ./cmd/genbench -dir testdata/pdfs -runs 1 -count 10 -model gemini-3.6-flash` (free tier, no fallback, 3-minute timeout per call). **Only 2 of 16 calls succeeded**: the model was overloaded for the whole run, so this says nothing yet about which strategy is better.

| PDF | Strategy | Result | Total | Input tok | Output tok | Thinking tok | Total tok |
|---|---|---|---|---|---|---|---|
| OT_NLP_Brunet_LLM_Lecture_2026.pdf | text-extract (61k chars) | ✅ 10 valid | 53.0 s | 21,195 | 2,096 | 3,045 | 26,336 |
| OT_NLP_Egyed_1_2026_V01.pdf (3.6 MB) | pdf-native | ✅ 10 valid | 56.6 s | 60,560 | 1,905 | 3,430 | 65,895 |

The other 14 calls failed:

| Failure | Calls | Time to fail |
|---|---|---|
| 503 "model is currently experiencing high demand" | 7 | 1.7–176 s |
| 504 "Deadline expired" (server side) | 4 | ~179 s |
| Client timeout (3 min) | 3 | 180 s |

What the run does show:
- On a large slide deck, `pdf-native` costs far more input tokens: 60.6k for Egyed 1 as a PDF, against 21k for Brunet as text. These are different files, so this is a hint to confirm, not a measurement.
- A successful call on a real lecture takes about a minute, and a failed one can take the full 3 minutes. With `JOB_TIMEOUT=3m`, a slow failure leaves no time for fallback models.

Next: rerun when the model isn't overloaded, with `-runs 3`, and add a scanned PDF.

## Decision
Default `GENERATION_STRATEGY=pdf-native` until results say otherwise (it is the only strategy that works on scanned PDFs).
