package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"path/filepath"
	"strings"
)

// MaxDocumentBytes caps uploads (inline Gemini requests allow ~20 MB).
const MaxDocumentBytes = 15 << 20

// PDFMediaType is the only accepted document type.
const PDFMediaType = "application/pdf"

// Document is an uploaded PDF held in memory for processing only.
// It is a value object and is NEVER persisted: there is no repository for it.
// Only its DocumentMeta is stored, on the ProcessingJob.
type Document struct {
	filename string
	data     []byte
	sha256   string
}

// DocumentMeta is the persisted, content-free description of a Document.
type DocumentMeta struct {
	Filename string
	Size     int64
	SHA256   string
}

// NewDocument validates that data is a PDF within the size limit.
func NewDocument(filename string, data []byte) (Document, error) {
	var problems []string
	filename = strings.TrimSpace(filepath.Base(filename))
	if filename == "" || filename == "." || filename == "/" {
		filename = "document.pdf"
	}
	switch {
	case len(data) == 0:
		problems = append(problems, "document is empty")
	case len(data) > MaxDocumentBytes:
		problems = append(problems, "document must be at most 15 MB")
	case !bytes.HasPrefix(data, []byte("%PDF-")):
		problems = append(problems, "document must be a PDF")
	}
	if err := newValidationError(problems); err != nil {
		return Document{}, err
	}
	sum := sha256.Sum256(data)
	return Document{filename: filename, data: data, sha256: hex.EncodeToString(sum[:])}, nil
}

// Filename returns the sanitized original filename.
func (d Document) Filename() string { return d.filename }

// Bytes returns the raw PDF. Callers must not retain or persist it.
func (d Document) Bytes() []byte { return d.data }

// MediaType returns the document's MIME type.
func (d Document) MediaType() string { return PDFMediaType }

// Size returns the document size in bytes.
func (d Document) Size() int64 { return int64(len(d.data)) }

// SHA256 returns the hex digest of the content.
func (d Document) SHA256() string { return d.sha256 }

// Meta returns the persistable metadata.
func (d Document) Meta() DocumentMeta {
	return DocumentMeta{Filename: d.filename, Size: d.Size(), SHA256: d.sha256}
}

// LogValue keeps document content out of logs even if logged directly.
func (d Document) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("filename", d.filename),
		slog.Int64("size", d.Size()),
		slog.String("sha256", d.sha256),
	)
}
