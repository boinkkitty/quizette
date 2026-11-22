package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/boinkkitty/quizette/backend/internal/domain"
	"github.com/boinkkitty/quizette/backend/internal/platform/logctx"
)

// SessionTTL is how long a login lasts.
const SessionTTL = 30 * 24 * time.Hour

// IdentityService handles sign-up, login, sessions and OAuth.
//
// A login creates a Session identified by an opaque random token. The client
// holds the token in an HttpOnly cookie; only its SHA-256 hash is stored, so a
// database leak does not expose live sessions.
type IdentityService struct {
	Deps
	Hasher    PasswordHasher
	Providers map[string]OAuthProvider
	// dummyHash is compared against on unknown emails to equalise timing.
	dummyOnce sync.Once
	dummyHash string
}

// NewSessionToken is a new session's raw token, to send to the client, and its expiry.
type NewSessionToken struct {
	Token     logctx.Secret
	ExpiresAt time.Time
}

// SignUp creates a password account and logs it in.
func (s *IdentityService) SignUp(ctx context.Context, email, password, name string) (domain.User, NewSessionToken, error) {
	email, err := domain.NormalizeEmail(email)
	if err != nil {
		return domain.User{}, NewSessionToken{}, err
	}
	if err := domain.ValidatePassword(password); err != nil {
		return domain.User{}, NewSessionToken{}, err
	}
	hash, err := s.Hasher.Hash(password)
	if err != nil {
		return domain.User{}, NewSessionToken{}, fmt.Errorf("hash password: %w", err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	u := domain.User{ID: s.IDs.NewID(), Email: email, PasswordHash: hash, Name: name, CreatedAt: s.Clock.Now()}
	if err := s.Repos.Users.Create(ctx, u); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return domain.User{}, NewSessionToken{}, fmt.Errorf("%w: an account with this email already exists", domain.ErrConflict)
		}
		return domain.User{}, NewSessionToken{}, err
	}
	tok, err := s.createSession(ctx, u.ID)
	if err != nil {
		return domain.User{}, NewSessionToken{}, err
	}
	logctx.From(ctx, s.Logger).InfoContext(ctx, "user.signed_up", slog.String("user_id", u.ID.String()), slog.String("method", "password"))
	return u, tok, nil
}

// LogIn verifies a password and creates a session.
func (s *IdentityService) LogIn(ctx context.Context, email, password string) (domain.User, NewSessionToken, error) {
	email, _ = domain.NormalizeEmail(email)
	u, err := s.Repos.Users.GetByEmail(ctx, email)
	if err != nil && !isNotFound(err) {
		return domain.User{}, NewSessionToken{}, err
	}
	if err != nil || !u.HasPassword() {
		s.Hasher.Compare(s.dummy(), password)
		return domain.User{}, NewSessionToken{}, fmt.Errorf("%w: invalid email or password", domain.ErrUnauthorized)
	}
	if !s.Hasher.Compare(u.PasswordHash, password) {
		return domain.User{}, NewSessionToken{}, fmt.Errorf("%w: invalid email or password", domain.ErrUnauthorized)
	}
	tok, err := s.createSession(ctx, u.ID)
	if err != nil {
		return domain.User{}, NewSessionToken{}, err
	}
	return u, tok, nil
}

func (s *IdentityService) dummy() string {
	s.dummyOnce.Do(func() { s.dummyHash, _ = s.Hasher.Hash("dummy-password-for-timing") })
	return s.dummyHash
}

// LogOut deletes the session for token. An empty token is a no-op.
func (s *IdentityService) LogOut(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.Repos.Sessions.Delete(ctx, hashToken(token))
}

// Authenticate resolves a session token to its user. Unknown, expired or
// orphaned sessions yield ErrUnauthorized; expired ones are deleted.
func (s *IdentityService) Authenticate(ctx context.Context, token string) (domain.User, error) {
	if token == "" {
		return domain.User{}, domain.ErrUnauthorized
	}
	sess, err := s.Repos.Sessions.Get(ctx, hashToken(token))
	if err != nil {
		if isNotFound(err) {
			return domain.User{}, domain.ErrUnauthorized
		}
		return domain.User{}, err
	}
	if !s.Clock.Now().Before(sess.ExpiresAt) {
		_ = s.Repos.Sessions.Delete(ctx, sess.TokenHash)
		return domain.User{}, domain.ErrUnauthorized
	}
	u, err := s.Repos.Users.GetByID(ctx, sess.UserID)
	if isNotFound(err) {
		return domain.User{}, domain.ErrUnauthorized
	}
	return u, err
}

// OAuthStart is the redirect URL plus values the caller must keep (in a
// short-lived cookie) until the callback.
type OAuthStart struct {
	URL      string
	State    string
	Verifier logctx.Secret
}

// StartOAuth begins an OAuth flow.
func (s *IdentityService) StartOAuth(provider string) (OAuthStart, error) {
	p, ok := s.Providers[provider]
	if !ok {
		return OAuthStart{}, fmt.Errorf("%w: unknown provider %q", domain.ErrNotFound, provider)
	}
	state, err := randomToken(24)
	if err != nil {
		return OAuthStart{}, err
	}
	verifier, err := randomToken(32)
	if err != nil {
		return OAuthStart{}, err
	}
	return OAuthStart{URL: p.AuthCodeURL(state, verifier), State: state, Verifier: logctx.Secret(verifier)}, nil
}

// CompleteOAuth exchanges the code, finds or creates the user, and logs in.
// An existing password account is linked only when the provider verified the email.
func (s *IdentityService) CompleteOAuth(ctx context.Context, provider, code, verifier string) (domain.User, NewSessionToken, error) {
	p, ok := s.Providers[provider]
	if !ok {
		return domain.User{}, NewSessionToken{}, fmt.Errorf("%w: unknown provider %q", domain.ErrNotFound, provider)
	}
	ident, err := p.Exchange(ctx, code, verifier)
	if err != nil {
		return domain.User{}, NewSessionToken{}, fmt.Errorf("%w: oauth exchange failed: %w", domain.ErrUnauthorized, err)
	}

	var u domain.User
	err = s.UoW.Do(ctx, func(r Repos) error {
		var err error
		u, err = r.Users.GetByOAuth(ctx, provider, ident.ProviderUserID)
		if err == nil || !isNotFound(err) {
			return err
		}
		email, err := domain.NormalizeEmail(ident.Email)
		if err != nil {
			return domain.Invalid("your " + provider + " account has no usable email address")
		}
		existing, err := r.Users.GetByEmail(ctx, email)
		switch {
		case err == nil && ident.EmailVerified:
			u = existing
		case err == nil:
			return fmt.Errorf("%w: an account with this email already exists; log in with your password", domain.ErrConflict)
		case isNotFound(err):
			name := strings.TrimSpace(ident.Name)
			if name == "" {
				name = strings.Split(email, "@")[0]
			}
			u = domain.User{ID: s.IDs.NewID(), Email: email, Name: name, AvatarURL: ident.AvatarURL, CreatedAt: s.Clock.Now()}
			if err := r.Users.Create(ctx, u); err != nil {
				return err
			}
		default:
			return err
		}
		return r.Users.LinkOAuth(ctx, u.ID, provider, ident.ProviderUserID)
	})
	if err != nil {
		return domain.User{}, NewSessionToken{}, err
	}
	tok, err := s.createSession(ctx, u.ID)
	if err != nil {
		return domain.User{}, NewSessionToken{}, err
	}
	logctx.From(ctx, s.Logger).InfoContext(ctx, "user.oauth_login", slog.String("user_id", u.ID.String()), slog.String("provider", provider))
	return u, tok, nil
}

// createSession stores a new session for userID and returns its raw token.
func (s *IdentityService) createSession(ctx context.Context, userID domain.ID) (NewSessionToken, error) {
	token, err := randomToken(32)
	if err != nil {
		return NewSessionToken{}, err
	}
	now := s.Clock.Now()
	sess := Session{TokenHash: hashToken(token), UserID: userID, CreatedAt: now, ExpiresAt: now.Add(SessionTTL)}
	if err := s.Repos.Sessions.Create(ctx, sess); err != nil {
		return NewSessionToken{}, err
	}
	return NewSessionToken{Token: logctx.Secret(token), ExpiresAt: sess.ExpiresAt}, nil
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// PurgeExpiredSessions deletes expired sessions.
func (s *IdentityService) PurgeExpiredSessions(ctx context.Context) (int64, error) {
	return s.Repos.Sessions.DeleteExpired(ctx, s.Clock.Now())
}

// ProviderNames lists configured OAuth providers.
func (s *IdentityService) ProviderNames() []string {
	names := make([]string, 0, len(s.Providers))
	for n := range s.Providers {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}
