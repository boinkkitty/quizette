// Command api is the Quizette HTTP API and composition root: it reads
// configuration, builds adapters and injects them into the app services.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	qhttp "github.com/boinkkitty/quizette/backend/internal/adapters/http"
	"github.com/boinkkitty/quizette/backend/internal/adapters/llm/gemini"
	"github.com/boinkkitty/quizette/backend/internal/adapters/llm/stub"
	"github.com/boinkkitty/quizette/backend/internal/adapters/oauth"
	"github.com/boinkkitty/quizette/backend/internal/adapters/pdf/ledongthuc"
	"github.com/boinkkitty/quizette/backend/internal/adapters/postgres"
	"github.com/boinkkitty/quizette/backend/internal/adapters/queue/inprocess"
	"github.com/boinkkitty/quizette/backend/internal/adapters/system"
	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/config"
	"github.com/boinkkitty/quizette/backend/internal/platform/logging"
)

func main() {
	if err := run(); err != nil {
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("fatal", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	format := logging.Text
	if cfg.Production() {
		format = logging.JSON
	}
	logger := logging.New(logging.Options{
		Writer: os.Stdout, Format: format, Level: cfg.LogLevel,
		Service: "quizette-api", Env: cfg.Env, Version: cfg.Version,
	})
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool, logger); err != nil {
		return err
	}
	store := postgres.NewStore(pool)

	generator, err := newGenerator(ctx, cfg)
	if err != nil {
		return err
	}

	deps := app.Deps{Repos: store.Repos(), UoW: store, Clock: system.Clock{}, IDs: system.UUIDv7{}, Logger: logger}
	genSvc := &app.GenerationService{
		Deps: deps, Generator: generator,
		Config: app.GenerationConfig{MaxRetries: 1, JobTimeout: cfg.JobTimeout},
	}
	queue := inprocess.New(cfg.JobWorkers, cfg.JobQueueSize, genSvc.ProcessJob, logger)
	genSvc.Queue = queue
	if _, err := genSvc.RecoverInterrupted(ctx); err != nil {
		return fmt.Errorf("recover interrupted jobs: %w", err)
	}

	identity := &app.IdentityService{Deps: deps, Hasher: system.Bcrypt{}, Providers: oauthProviders(cfg)}
	go purgeSessions(ctx, identity, logger)

	handler := qhttp.NewRouter(qhttp.Config{
		Identity: identity, Library: &app.LibraryService{Deps: deps}, Generation: genSvc, Practice: &app.PracticeService{Deps: deps},
		Logger: logger, PublicURL: cfg.PublicURL, SecureCookies: cfg.Production(),
		Ready: pool.Ping,
	})
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute, // large uploads on slow links
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    64 << 10,
	}

	errc := make(chan error, 1)
	go func() {
		logger.InfoContext(ctx, "http.listening", slog.String("addr", srv.Addr), slog.String("llm_provider", cfg.LLMProvider), slog.String("strategy", cfg.GenerationStrategy), slog.String("model", cfg.GeminiModel), slog.Any("fallback_models", cfg.GeminiFallbackModels), slog.Any("oauth", identity.ProviderNames()))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()

	select {
	case err := <-errc:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
	case <-ctx.Done():
	}

	logger.Info("shutdown.started")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown.http_failed", slog.Any("error", err))
	}
	queue.Shutdown(shutdownCtx)
	logger.Info("shutdown.complete")
	return nil
}

func newGenerator(ctx context.Context, cfg config.Config) (app.QuestionGenerator, error) {
	if cfg.LLMProvider == "stub" {
		return stub.Generator{Delay: cfg.StubLLMDelay}, nil
	}
	g, err := gemini.New(ctx, gemini.Config{
		APIKey: cfg.GeminiAPIKey, Model: cfg.GeminiModel, FallbackModels: cfg.GeminiFallbackModels, Strategy: cfg.GenerationStrategy,
		Extractor: ledongthuc.Extractor{}, ThinkingBudget: cfg.GeminiThinkingBudget,
	})
	if err != nil {
		return nil, fmt.Errorf("llm: %w", err)
	}
	return g, nil
}

func oauthProviders(cfg config.Config) map[string]app.OAuthProvider {
	providers := map[string]app.OAuthProvider{}
	redirect := func(p string) string { return cfg.PublicURL + "/api/auth/" + p + "/callback" }
	if cfg.GoogleClientID != "" {
		providers["google"] = oauth.Google(oauth.Credentials{ClientID: cfg.GoogleClientID, ClientSecret: cfg.GoogleClientSecret, RedirectURL: redirect("google")})
	}
	if cfg.GitHubClientID != "" {
		providers["github"] = oauth.GitHub(oauth.Credentials{ClientID: cfg.GitHubClientID, ClientSecret: cfg.GitHubClientSecret, RedirectURL: redirect("github")})
	}
	return providers
}

func purgeSessions(ctx context.Context, identity *app.IdentityService, logger *slog.Logger) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := identity.PurgeExpiredSessions(ctx); err != nil {
				logger.WarnContext(ctx, "sessions.purge_failed", slog.Any("error", err))
			} else if n > 0 {
				logger.InfoContext(ctx, "sessions.purged", slog.Int64("count", n))
			}
		}
	}
}
