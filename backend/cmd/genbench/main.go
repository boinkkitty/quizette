// Command genbench compares question-generation strategies (PDF sent natively
// vs. text extracted with ledongthuc/pdf) on real Gemini calls, measuring
// latency, token usage and validation pass rate.
//
//	GEMINI_API_KEY=... go run ./cmd/genbench -dir ./testdata/pdfs -runs 3 -count 10
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/adapters/llm/gemini"
	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/ledongthuc"
	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

type row struct {
	File      string
	Strategy  string
	Run       int
	OK        bool
	Valid     int
	Discarded int
	Err       string
	domain.GenerationMetrics
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "genbench:", err)
		os.Exit(1)
	}
}

func run(stdout io.Writer) error {
	dir := flag.String("dir", "testdata/pdfs", "directory of PDF fixtures")
	runs := flag.Int("runs", 3, "runs per file and strategy")
	count := flag.Int("count", 10, "questions per run (5-30, multiple of 5)")
	strategies := flag.String("strategies", domain.StrategyPDFNative+","+domain.StrategyTextExtract, "comma-separated strategies")
	model := flag.String("model", gemini.DefaultModel, "Gemini model")
	sleep := flag.Duration("sleep", 7*time.Second, "pause between calls (free tier is ~10 requests/minute)")
	csvPath := flag.String("csv", "genbench-results.csv", "CSV output path ('' to skip)")
	timeout := flag.Duration("timeout", 3*time.Minute, "per-call timeout")
	flag.Parse()

	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return errors.New("GEMINI_API_KEY is required")
	}
	spec, err := domain.NewQuestionSpec(*count, nil)
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(*dir, "*.pdf"))
	if err != nil || len(files) == 0 {
		return fmt.Errorf("no PDFs found in %s", *dir)
	}
	sort.Strings(files)

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	gens := map[string]*gemini.Generator{}
	strats := strings.Split(*strategies, ",")
	for _, s := range strats {
		g, err := gemini.New(ctx, gemini.Config{APIKey: key, Model: *model, Strategy: s, Extractor: ledongthuc.Extractor{}})
		if err != nil {
			return err
		}
		gens[s] = g
	}

	var rows []row
	first := true
	for _, f := range files {
		data, err := os.ReadFile(f) //nolint:gosec // operator-supplied fixture path
		if err != nil {
			return err
		}
		doc, err := domain.NewDocument(filepath.Base(f), data)
		if err != nil {
			logger.Warn("skip", slog.String("file", f), slog.Any("error", err))
			continue
		}
		for i := 1; i <= *runs; i++ {
			for _, s := range strats {
				if !first {
					time.Sleep(*sleep)
				}
				first = false
				r := measure(ctx, *timeout, gens[s], doc, spec)
				r.File, r.Strategy, r.Run = doc.Filename(), s, i
				logger.Info("run", slog.String("file", r.File), slog.String("strategy", s), slog.Int("run", i),
					slog.Bool("ok", r.OK), slog.Int64("total_ms", r.TotalMS), slog.Int("total_tokens", r.TotalTokens), slog.String("error", r.Err))
				rows = append(rows, r)
			}
		}
	}

	if *csvPath != "" {
		if err := writeCSV(*csvPath, rows); err != nil {
			return err
		}
	}
	writeMarkdown(stdout, rows, strats, *model, *count, *runs)
	return nil
}

func measure(ctx context.Context, timeout time.Duration, g *gemini.Generator, doc domain.Document, spec domain.QuestionSpec) row {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := g.Generate(ctx, app.GenerateInput{Document: doc, Spec: spec})
	r := row{GenerationMetrics: res.Metrics}
	if err != nil {
		r.Err = err.Error()
		return r
	}
	n := 0
	set, discarded, err := domain.NewQuestionSet("bench", "bench", spec, res.Drafts, func() domain.ID { n++; return domain.ID(strconv.Itoa(n)) }, time.Now())
	r.Discarded = len(discarded)
	if err != nil {
		r.Err = "validation: " + err.Error()
		return r
	}
	r.OK, r.Valid = true, set.Size()
	return r
}

func writeCSV(path string, rows []row) error {
	f, err := os.Create(path) //nolint:gosec // operator-chosen output path
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"file", "strategy", "run", "ok", "valid", "discarded", "input_tokens", "output_tokens", "thinking_tokens", "total_tokens", "extract_ms", "llm_ms", "total_ms", "extracted_chars", "error"})
	for _, r := range rows {
		_ = w.Write([]string{
			r.File, r.Strategy, strconv.Itoa(r.Run), strconv.FormatBool(r.OK), strconv.Itoa(r.Valid), strconv.Itoa(r.Discarded),
			strconv.Itoa(r.InputTokens), strconv.Itoa(r.OutputTokens), strconv.Itoa(r.ThinkingTokens), strconv.Itoa(r.TotalTokens),
			strconv.FormatInt(r.ExtractMS, 10), strconv.FormatInt(r.LLMMS, 10), strconv.FormatInt(r.TotalMS, 10), strconv.Itoa(r.ExtractedChars), r.Err,
		})
	}
	w.Flush()
	return w.Error()
}

func percentile(vals []int64, p float64) int64 {
	if len(vals) == 0 {
		return 0
	}
	s := slices.Clone(vals)
	slices.Sort(s)
	idx := int(float64(len(s)-1) * p)
	return s[idx]
}

func mean(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	sum := 0
	for _, v := range vals {
		sum += v
	}
	return sum / len(vals)
}

type agg struct {
	total, extract, llm         []int64
	in, out, think, tokens, chr []int
	ok, n                       int
}

func (a *agg) add(r row) {
	a.n++
	if r.OK {
		a.ok++
	}
	a.total = append(a.total, r.TotalMS)
	a.extract = append(a.extract, r.ExtractMS)
	a.llm = append(a.llm, r.LLMMS)
	a.in = append(a.in, r.InputTokens)
	a.out = append(a.out, r.OutputTokens)
	a.think = append(a.think, r.ThinkingTokens)
	a.tokens = append(a.tokens, r.TotalTokens)
	a.chr = append(a.chr, r.ExtractedChars)
}

func (a *agg) line(label string) string {
	return fmt.Sprintf("| %s | %d/%d | %d | %d | %d | %d | %d | %d | %d | %d |", label, a.ok, a.n,
		percentile(a.total, 0.5), percentile(a.total, 0.95), percentile(a.extract, 0.5), percentile(a.llm, 0.5),
		mean(a.in), mean(a.out), mean(a.think), mean(a.tokens))
}

func writeMarkdown(w io.Writer, rows []row, strats []string, model string, count, runs int) {
	fmt.Fprintf(w, "## Generation strategy benchmark\n\nModel `%s`, %d questions per run, %d runs per file. Times in ms (p50 unless noted); tokens are means.\n\n", model, count, runs)
	header := "| %s | pass | total p50 | total p95 | extract p50 | llm p50 | input tok | output tok | thinking tok | total tok |\n|---|---|---|---|---|---|---|---|---|---|\n"

	fmt.Fprintf(w, "### Overall\n\n"+header, "strategy")
	for _, s := range strats {
		a := &agg{}
		for _, r := range rows {
			if r.Strategy == s {
				a.add(r)
			}
		}
		fmt.Fprintln(w, a.line(s))
	}

	fmt.Fprintf(w, "\n### Per file\n\n"+header, "file / strategy")
	var files []string
	for _, r := range rows {
		if !slices.Contains(files, r.File) {
			files = append(files, r.File)
		}
	}
	for _, f := range files {
		for _, s := range strats {
			a := &agg{}
			for _, r := range rows {
				if r.File == f && r.Strategy == s {
					a.add(r)
				}
			}
			fmt.Fprintln(w, a.line(f+" / "+s))
		}
	}

	var failures []string
	for _, r := range rows {
		if r.Err != "" {
			failures = append(failures, fmt.Sprintf("- %s / %s run %d: %.200s", r.File, r.Strategy, r.Run, r.Err))
		}
	}
	if len(failures) > 0 {
		fmt.Fprintf(w, "\n### Failures\n\n%s\n", strings.Join(failures, "\n"))
	}
}
