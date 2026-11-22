-- Users, linked OAuth accounts and sessions.

-- name: CreateUser :exec
INSERT INTO users (id, email, password_hash, name, avatar_url, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- email is citext, so this match is case-insensitive.
-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByOAuth :one
SELECT u.* FROM users u
JOIN oauth_accounts o ON o.user_id = u.id
WHERE o.provider = $1 AND o.provider_user_id = $2;

-- Idempotent: linking an already-linked provider account is a no-op.
-- name: LinkOAuth :exec
INSERT INTO oauth_accounts (user_id, provider, provider_user_id)
VALUES ($1, $2, $3)
ON CONFLICT (provider, provider_user_id) DO NOTHING;

-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES ($1, $2, $3, $4);

-- name: GetSession :one
SELECT * FROM sessions WHERE token_hash = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at < $1;
