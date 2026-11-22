package http

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/platform/logctx"
)

const (
	// sessionCookie holds the opaque session token for every API request.
	sessionCookie = "qz_session"
	oauthCookie   = "qz_oauth"
)

func (s *Server) setSessionCookie(w http.ResponseWriter, tok app.NewSessionToken) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is configured (off only for local http)
		Name: sessionCookie, Value: string(tok.Token), Path: "/", Expires: tok.ExpiresAt,
		HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is configured (off only for local http)
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name,omitempty"`
}

func (s *Server) signUp(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	u, tok, err := s.cfg.Identity.SignUp(r.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.setSessionCookie(w, tok)
	writeJSON(w, http.StatusCreated, toUserDTO(u))
}

func (s *Server) logIn(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, r, err)
		return
	}
	u, tok, err := s.cfg.Identity.LogIn(r.Context(), req.Email, req.Password)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	s.setSessionCookie(w, tok)
	writeJSON(w, http.StatusOK, toUserDTO(u))
}

func (s *Server) logOut(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.cfg.Identity.LogOut(r.Context(), c.Value); err != nil {
			s.writeError(w, r, err)
			return
		}
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, ok := userFrom(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errorBody{Error: "unauthorized", Message: "not logged in"})
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(u))
}

func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	start, err := s.cfg.Identity.StartOAuth(provider)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is configured (off only for local http)
		Name: oauthCookie, Value: start.State + "." + string(start.Verifier), Path: "/api/auth/" + provider,
		MaxAge: int((10 * time.Minute).Seconds()), HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, start.URL, http.StatusFound)
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	fail := func(reason string) { //nolint:contextcheck // uses r.Context() of the enclosing handler
		logctx.From(r.Context(), s.log).WarnContext(r.Context(), "auth.oauth_failed", slog.String("provider", provider), slog.String("reason", reason))
		http.Redirect(w, r, s.cfg.PublicURL+"/login?error="+url.QueryEscape(reason), http.StatusFound)
	}
	c, err := r.Cookie(oauthCookie)
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is configured (off only for local http)
		Name: oauthCookie, Value: "", Path: "/api/auth/" + provider, MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
	if err != nil {
		fail("Your sign-in session expired. Please try again.")
		return
	}
	state, verifier, ok := strings.Cut(c.Value, ".")
	if !ok || state == "" || r.URL.Query().Get("state") != state {
		fail("Sign-in could not be verified. Please try again.")
		return
	}
	if e := r.URL.Query().Get("error"); e != "" {
		fail("Sign-in was cancelled.")
		return
	}
	_, tok, err := s.cfg.Identity.CompleteOAuth(r.Context(), provider, r.URL.Query().Get("code"), verifier)
	if err != nil {
		var msg string
		if strings.Contains(err.Error(), "already exists") {
			msg = "An account with this email already exists. Log in with your password."
		} else {
			msg = "Sign-in failed. Please try again."
		}
		logctx.From(r.Context(), s.log).WarnContext(r.Context(), "auth.oauth_error", slog.Any("error", err))
		fail(msg)
		return
	}
	s.setSessionCookie(w, tok)
	http.Redirect(w, r, s.cfg.PublicURL+"/", http.StatusFound)
}

func (s *Server) providers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string][]string{"providers": s.cfg.Identity.ProviderNames()})
}
