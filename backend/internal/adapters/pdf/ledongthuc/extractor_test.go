package ledongthuc_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/ledongthuc"
	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/pdftest"
	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

func TestExtract(t *testing.T) {
	t.Parallel()
	doc, err := domain.NewDocument("a.pdf", pdftest.Minimal("Mitochondria are the powerhouse", "Ribosomes make proteins"))
	if err != nil {
		t.Fatal(err)
	}
	text, err := ledongthuc.Extractor{}.Extract(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Mitochondria are the powerhouse", "Ribosomes make proteins", "Page 2"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %q", want, text)
		}
	}
}

func TestExtractCorrupt(t *testing.T) {
	t.Parallel()
	doc, err := domain.NewDocument("bad.pdf", []byte("%PDF-1.4 garbage with no xref"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = ledongthuc.Extractor{}.Extract(context.Background(), doc)
	if !errors.Is(err, app.ErrUnreadableDocument) {
		t.Fatalf("want ErrUnreadableDocument, got %v", err)
	}
}
