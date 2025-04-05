-- db/query/user.sql

-- name: CreateUser :one
INSERT INTO users (
  username,
  password_plaintext
) VALUES (
  $1, $2
) RETURNING *;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1 LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: UpdateUserStatus :exec
UPDATE users
SET status = $2
WHERE id = $1;

-- name: ListOnlineUsers :many
SELECT id, username FROM users
WHERE status = 'online'
ORDER BY username;

-- name: ListOfflineUsers :many
SELECT id, username FROM users
WHERE status = 'offline'
ORDER BY username;