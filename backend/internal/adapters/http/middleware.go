package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/boinkkitty/quizette/backend/internal/domain"
	"github.com/boinkkitty/quizette/backend/internal/platform/logctx"
)

type ctxKey int

const userKey ctxKey = iota

// CSRFHeader must accompany every state-changing API request. The session
// cookie is sent automatically, but browsers cannot add custom headers
// cross-site without CORS, which we never enable.
const CSRFHeader = "X-Requested-With"

func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 64 {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-ID", id)
		ctx := logctx.With(r.Context(), s.log.With(slog.String("request_id", id)))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err //nolint:wrapcheck // transparent wrapper
}

// errHolder lets handlers attach the internal error to the request log line.
type errHolder struct{ err error }

type errKey struct{}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		holder := &errHolder{}
		ctx := context.WithValue(r.Context(), errKey{}, holder)
		r = r.WithContext(ctx)
		next.ServeHTTP(rec, r)

		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		attrs := []slog.Attr{
			slog.String("method", r.Method),
			slog.String("route", route),
			slog.Int("status", rec.status),
			slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			slog.Int("bytes", rec.bytes),
		}
		if u, ok := userFrom(r.Context()); ok {
			attrs = append(attrs, slog.String("user_id", u.ID.String()))
		}
		level := slog.LevelInfo
		if rec.status >= 500 {
			level = slog.LevelError
			if holder.err != nil {
				attrs = append(attrs, slog.Any("error", holder.err))
			}
		}
		if route == "/healthz" {
			level = slog.LevelDebug
		}
		logctx.From(r.Context(), s.log).LogAttrs(r.Context(), level, "http.request", attrs...)
	})
}

func recordError(ctx context.Context, err error) {
	if h, ok := ctx.Value(errKey{}).(*errHolder); ok {
		h.err = err
	}
}

func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler { //nolint:errorlint // sentinel compared by identity per net/http docs
					panic(v)
				}
				logctx.From(r.Context(), s.log).ErrorContext(r.Context(), "http.panic", slog.Any("panic", v), slog.String("stack", string(debug.Stack())))
				writeJSON(w, http.StatusInternalServerError, errorBody{Error: "internal", Message: "something went wrong"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// authenticate resolves the session cookie, if any, to a user and stores it
// in the request context. Requests without a valid session continue
// anonymously; an invalid cookie is cleared.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		u, err := s.cfg.Identity.Authenticate(r.Context(), c.Value)
		switch {
		case errors.Is(err, domain.ErrUnauthorized):
			// Unknown or expired session: forget it and continue as a guest.
			s.clearSessionCookie(w)
			next.ServeHTTP(w, r)
			return
		case err != nil:
			// The session couldn't be checked (e.g. the database is unreachable). Keep the
			// cookie, which may well be valid, and ask the client to retry later.
			logctx.From(r.Context(), s.log).ErrorContext(r.Context(), "auth.session_lookup_failed", slog.Any("error", err))
			writeJSON(w, http.StatusServiceUnavailable, errorBody{Error: "unavailable", Message: "the service is temporarily unavailable; please try again"})
			return
		}
		ctx := context.WithValue(r.Context(), userKey, u)
		ctx = logctx.AddAttrs(ctx, s.log, slog.String("user_id", u.ID.String()))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// userFrom returns the user stored by authenticate.
func userFrom(ctx context.Context) (domain.User, bool) {
	u, ok := ctx.Value(userKey).(domain.User)
	return u, ok
}

// mustUser returns the user of a route behind requireUser.
func mustUser(r *http.Request) domain.User {
	u, _ := userFrom(r.Context())
	return u
}

// requireUser rejects requests without a logged-in user.
func requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := userFrom(r.Context()); !ok {
			writeJSON(w, http.StatusUnauthorized, errorBody{Error: "unauthorized", Message: "please log in"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get(CSRFHeader) == "" {
				writeJSON(w, http.StatusForbidden, errorBody{Error: "csrf", Message: "missing " + CSRFHeader + " header"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
