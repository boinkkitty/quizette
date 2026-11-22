package http

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/boinkkitty/quizette/backend/internal/domain"
	"github.com/boinkkitty/quizette/backend/internal/platform/logctx"
)

type errorBody struct {
	Error    string   `json:"error"`
	Message  string   `json:"message"`
	Problems []string `json:"problems,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// writeError maps domain errors to HTTP responses. Internal details are
// logged, never returned.
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "validation", Message: strings.Join(ve.Problems, "; "), Problems: ve.Problems})
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not_found", Message: "not found"})
	case errors.Is(err, domain.ErrConflict):
		writeJSON(w, http.StatusConflict, errorBody{Error: "conflict", Message: publicMessage(err, "conflict")})
	case errors.Is(err, domain.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "unauthorized", Message: publicMessage(err, "unauthorized")})
	default:
		recordError(r.Context(), err)
		logctx.From(r.Context(), s.log).ErrorContext(r.Context(), "http.internal_error", slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "internal", Message: "something went wrong"})
	}
}

// publicMessage returns the text after the sentinel ("conflict: <msg>").
func publicMessage(err error, prefix string) string {
	msg := err.Error()
	if i := strings.Index(msg, prefix+": "); i >= 0 {
		return msg[i+len(prefix)+2:]
	}
	return prefix
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return domain.Invalid("request body is not valid JSON")
	}
	return nil
}
