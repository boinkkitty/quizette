// Package http is the driving adapter exposing the app services as a JSON API.
package http

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/boinkkitty/quizette/backend/internal/app"
)

// Config wires the HTTP adapter.
type Config struct {
	Identity   *app.IdentityService
	Library    *app.LibraryService
	Generation *app.GenerationService
	Practice   *app.PracticeService
	Logger     *slog.Logger
	// PublicURL is the browser-facing origin (this server in production, the
	// Vite dev server locally), used for OAuth redirects back into the SPA.
	PublicURL string
	// SecureCookies sets the Secure flag (true everywhere except local http).
	SecureCookies bool
	// Ready reports dependency health for /readyz.
	Ready func(ctx context.Context) error
	// Web is the built frontend, served for every path outside /api. It must
	// contain index.html. Nil serves the API only (local development, where
	// the Vite dev server serves the frontend).
	Web fs.FS
}

// Server holds handler dependencies.
type Server struct {
	cfg Config
	log *slog.Logger
}

// NewRouter builds the router for the API and, when cfg.Web is set, the frontend.
func NewRouter(cfg Config) (http.Handler, error) {
	s := &Server{cfg: cfg, log: cfg.Logger}
	r := chi.NewRouter()
	r.Use(s.requestID, s.logRequests, s.recoverPanics)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", s.readyz)

	r.Route("/api", func(r chi.Router) {
		// Every API route is cookie-authenticated, so all of them need the CSRF guard.
		r.Use(s.authenticate, s.csrfGuard)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/signup", s.signUp)
			r.Post("/login", s.logIn)
			r.Post("/logout", s.logOut)
			r.Get("/me", s.me)
			r.Get("/providers", s.providers)
			r.Get("/{provider}/start", s.oauthStart)
			r.Get("/{provider}/callback", s.oauthCallback)
		})
		r.Group(func(r chi.Router) {
			r.Use(requireUser)
			r.Get("/topics", s.listTopics)
			r.Post("/topics", s.createTopic)
			r.Get("/topics/{id}", s.getTopic)
			r.Patch("/topics/{id}", s.renameTopic)
			r.Delete("/topics/{id}", s.deleteTopic)

			r.Post("/documents", s.uploadDocument)
			r.Get("/jobs/{id}", s.getJob)

			r.Get("/chapters", s.listChapters)
			r.Get("/chapters/{id}", s.getChapter)
			r.Patch("/chapters/{id}", s.renameChapter)
			r.Delete("/chapters/{id}", s.deleteChapter)
			r.Get("/chapters/{id}/question-set", s.getQuestionSet)
			r.Get("/chapters/{id}/attempts", s.listAttempts)

			r.Post("/quizzes/draw", s.drawQuiz)

			r.Get("/attempts", s.listAllAttempts)
			r.Post("/attempts", s.submitQuiz)
			r.Get("/attempts/{id}", s.getAttempt)
		})
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusNotFound, errorBody{Error: "not_found", Message: "no such endpoint"})
		})
	})

	if cfg.Web != nil {
		app, err := newSPA(cfg.Web)
		if err != nil {
			return nil, fmt.Errorf("frontend: %w", err)
		}
		r.Get("/*", app.ServeHTTP)
		r.Head("/*", app.ServeHTTP)
	}
	return r, nil
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Ready != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := s.cfg.Ready(ctx); err != nil {
			s.log.WarnContext(ctx, "readyz.failed", slog.Any("error", err))
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
