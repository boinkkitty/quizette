package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/boinkkitty/quizette/backend/internal/platform/logging"
)

func TestNewJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := logging.New(logging.Options{Writer: &buf, Format: logging.JSON, Level: slog.LevelInfo, Service: "api", Env: "production", Version: "abc123"})
	logger.Debug("hidden")
	logger.Info("job.started", slog.String("job_id", "j1"))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line (debug filtered out), got %d:\n%s", len(lines), buf.String())
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	for k, want := range map[string]string{"msg": "job.started", "service": "api", "env": "production", "version": "abc123", "job_id": "j1"} {
		if rec[k] != want {
			t.Errorf("%s = %v, want %q", k, rec[k], want)
		}
	}
}

func TestNewTextOmitsEmptyAttrs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logging.New(logging.Options{Writer: &buf, Level: slog.LevelDebug, Env: "development"}).Debug("hello")
	out := buf.String()
	if !strings.Contains(out, "msg=hello") || !strings.Contains(out, "env=development") {
		t.Errorf("text output missing fields: %q", out)
	}
	if strings.Contains(out, "service=") || strings.Contains(out, "version=") {
		t.Errorf("empty attributes should be omitted: %q", out)
	}
}
