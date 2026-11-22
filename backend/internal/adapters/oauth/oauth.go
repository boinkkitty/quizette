// Package oauth implements app.OAuthProvider for Google and GitHub using
// golang.org/x/oauth2 with PKCE.
package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
)

// Credentials are a provider's client ID/secret and redirect URL.
type Credentials struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type provider struct {
	name    string
	cfg     *oauth2.Config
	profile func(ctx context.Context, client *http.Client) (domain.OAuthIdentity, error)
}

func (p provider) Name() string { return p.name }

func (p provider) AuthCodeURL(state, verifier string) string {
	return p.cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
}

func (p provider) Exchange(ctx context.Context, code, verifier string) (domain.OAuthIdentity, error) {
	tok, err := p.cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return domain.OAuthIdentity{}, fmt.Errorf("%s token exchange: %w", p.name, err)
	}
	id, err := p.profile(ctx, p.cfg.Client(ctx, tok))
	if err != nil {
		return domain.OAuthIdentity{}, err
	}
	id.Provider = p.name
	return id, nil
}

// Google returns a Google OpenID Connect provider.
func Google(c Credentials) app.OAuthProvider {
	return provider{
		name: "google",
		cfg: &oauth2.Config{
			ClientID: c.ClientID, ClientSecret: c.ClientSecret, RedirectURL: c.RedirectURL,
			Endpoint: endpoints.Google, Scopes: []string{"openid", "email", "profile"},
		},
		profile: func(ctx context.Context, client *http.Client) (domain.OAuthIdentity, error) {
			var info struct {
				Sub           string `json:"sub"`
				Email         string `json:"email"`
				EmailVerified bool   `json:"email_verified"`
				Name          string `json:"name"`
				Picture       string `json:"picture"`
			}
			if err := getJSON(ctx, client, "https://openidconnect.googleapis.com/v1/userinfo", &info); err != nil {
				return domain.OAuthIdentity{}, err
			}
			if info.Sub == "" {
				return domain.OAuthIdentity{}, errors.New("google: missing subject")
			}
			return domain.OAuthIdentity{ProviderUserID: info.Sub, Email: info.Email, EmailVerified: info.EmailVerified, Name: info.Name, AvatarURL: info.Picture}, nil
		},
	}
}

// GitHub returns a GitHub OAuth provider. The email is the primary verified one.
func GitHub(c Credentials) app.OAuthProvider {
	return provider{
		name: "github",
		cfg: &oauth2.Config{
			ClientID: c.ClientID, ClientSecret: c.ClientSecret, RedirectURL: c.RedirectURL,
			Endpoint: endpoints.GitHub, Scopes: []string{"read:user", "user:email"},
		},
		profile: func(ctx context.Context, client *http.Client) (domain.OAuthIdentity, error) {
			var user struct {
				ID        int64  `json:"id"`
				Login     string `json:"login"`
				Name      string `json:"name"`
				AvatarURL string `json:"avatar_url"`
			}
			if err := getJSON(ctx, client, "https://api.github.com/user", &user); err != nil {
				return domain.OAuthIdentity{}, err
			}
			var emails []struct {
				Email    string `json:"email"`
				Primary  bool   `json:"primary"`
				Verified bool   `json:"verified"`
			}
			if err := getJSON(ctx, client, "https://api.github.com/user/emails", &emails); err != nil {
				return domain.OAuthIdentity{}, err
			}
			id := domain.OAuthIdentity{ProviderUserID: strconv.FormatInt(user.ID, 10), Name: user.Name, AvatarURL: user.AvatarURL}
			if id.Name == "" {
				id.Name = user.Login
			}
			for _, e := range emails {
				if e.Primary && e.Verified {
					id.Email, id.EmailVerified = e.Email, true
				}
			}
			return id, nil
		},
	}
}

func getJSON(ctx context.Context, client *http.Client, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("get %s: status %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(v); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}
