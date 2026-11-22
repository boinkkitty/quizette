// Package ledongthuc implements app.TextExtractor with github.com/ledongthuc/pdf.
package ledongthuc

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// Extractor extracts plain text page by page.
type Extractor struct{}

// Extract implements app.TextExtractor. The library can panic on malformed
// input, so panics are converted to errors.
func (Extractor) Extract(ctx context.Context, doc domain.Document) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: pdf parser panic: %v", app.ErrUnreadableDocument, r)
		}
	}()
	data := doc.Bytes()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("%w: %w", app.ErrUnreadableDocument, err)
	}
	var b strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		if err := ctx.Err(); err != nil {
			return "", fmt.Errorf("extract: %w", err)
		}
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		t, err := p.GetPlainText(nil)
		if err != nil {
			continue // skip unreadable pages, keep the rest
		}
		fmt.Fprintf(&b, "\n--- Page %d ---\n%s", i, t)
	}
	return b.String(), nil
}
