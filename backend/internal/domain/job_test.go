package domain

import (
	"errors"
	"testing"
	"time"
)

func TestProcessingJobTransitions(t *testing.T) {
	t.Parallel()
	now := time.Now()
	newJob := func() ProcessingJob {
		spec, _ := NewQuestionSpec(5, nil)
		return NewProcessingJob("j", "c", "u", spec, DocumentMeta{}, now)
	}

	j := newJob()
	if err := j.Succeed(now, GenerationMetrics{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("queued->succeeded: want ErrInvalidTransition, got %v", err)
	}
	if err := j.Start(now); err != nil || j.Status != JobRunning || j.Attempts != 1 || j.StartedAt == nil {
		t.Fatalf("start: %v %+v", err, j)
	}
	if err := j.Start(now); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("running->running: %v", err)
	}
	if err := j.Succeed(now, GenerationMetrics{TotalTokens: 10}); err != nil || j.Status != JobSucceeded || j.Metrics.TotalTokens != 10 {
		t.Fatalf("succeed: %v %+v", err, j)
	}
	if err := j.Fail(now, "x", GenerationMetrics{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("succeeded->failed: %v", err)
	}

	q := newJob()
	if err := q.Fail(now, "restart", GenerationMetrics{}); err != nil || q.Status != JobFailed || q.Error != "restart" {
		t.Fatalf("queued->failed: %v %+v", err, q)
	}
	if !q.IsTerminal() {
		t.Error("failed should be terminal")
	}
}

func TestChapterTransitions(t *testing.T) {
	t.Parallel()
	c, err := NewChapter("c", "t", "u", "  Intro ", time.Now())
	if err != nil || c.Title != "Intro" || c.Status != ChapterPending {
		t.Fatalf("%v %+v", err, c)
	}
	if err := c.MarkReady(); err != nil {
		t.Fatal(err)
	}
	if err := c.MarkFailed(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("ready->failed: %v", err)
	}
	if _, err := NewChapter("c", "t", "u", " ", time.Now()); err == nil {
		t.Error("empty title: want error")
	}
}

func TestNewDocument(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{"pdf", []byte("%PDF-1.7 rest"), false},
		{"empty", nil, true},
		{"not pdf", []byte("PK\x03\x04"), true},
		{"too large", append([]byte("%PDF-"), make([]byte, MaxDocumentBytes)...), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d, err := NewDocument("../../etc/notes.pdf", tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
			if err == nil && (d.Filename() != "notes.pdf" || len(d.SHA256()) != 64) {
				t.Errorf("meta = %+v", d.Meta())
			}
		})
	}
}
