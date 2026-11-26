// Package postgres implements the app repositories and UnitOfWork on
// PostgreSQL using pgx and sqlc-generated queries.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/boinkkitty/quizette/backend/internal/adapters/postgres/sqlcgen"
	"github.com/boinkkitty/quizette/backend/internal/app"
	"github.com/boinkkitty/quizette/backend/internal/domain"
	"github.com/boinkkitty/quizette/backend/migrations"
)

// Connect opens a connection pool and verifies connectivity.
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	// Neon's pooled endpoint (PgBouncer, transaction mode) does not support
	// server-side prepared statements across transactions.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate applies all pending migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	for _, r := range results {
		logger.InfoContext(ctx, "db.migrated", slog.Int64("version", r.Source.Version), slog.Int64("duration_ms", r.Duration.Milliseconds()))
	}
	return nil
}

// Store implements app.UnitOfWork and provides pool-backed repositories.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Repos returns non-transactional repositories.
func (s *Store) Repos() app.Repos { return reposFor(sqlcgen.New(s.pool), s.pool) }

// Do runs fn in a transaction, committing on success.
func (s *Store) Do(ctx context.Context, fn func(app.Repos) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { //nolint:wrapcheck // fn returns domain errors; keep them unwrapped
		return fn(reposFor(sqlcgen.New(tx), tx))
	})
}

// beginner starts a transaction or a nested savepoint.
type beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

func reposFor(q *sqlcgen.Queries, db beginner) app.Repos {
	return app.Repos{
		Topics:       topicRepo{q, db},
		Chapters:     chapterRepo{q},
		Jobs:         jobRepo{q},
		QuestionSets: setRepo{q, db},
		Attempts:     attemptRepo{q, db},
		Users:        userRepo{q},
		Sessions:     sessionRepo{q},
	}
}

// inTx runs fn atomically: inside an existing tx this creates a savepoint.
func inTx(ctx context.Context, db beginner, fn func(q *sqlcgen.Queries) error) error {
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error { return fn(sqlcgen.New(tx)) }) //nolint:wrapcheck // fn returns mapped errors
}

// mapErr translates driver errors into domain errors.
func mapErr(what string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", what, domain.ErrNotFound)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%s already exists: %w", what, domain.ErrConflict)
		case "22P02": // invalid_text_representation
			if strings.Contains(pgErr.Message, "uuid") { // malformed ID in a URL
				return fmt.Errorf("%s: %w", what, domain.ErrNotFound)
			}
		case "23503": // foreign_key_violation
			return fmt.Errorf("%s: related record missing: %w", what, domain.ErrNotFound)
		}
	}
	return fmt.Errorf("%s: %w", what, err)
}

func mustAffect(what string, n int64, err error) error {
	if err != nil {
		return mapErr(what, err)
	}
	if n == 0 {
		return fmt.Errorf("%s: %w", what, domain.ErrNotFound)
	}
	return nil
}
