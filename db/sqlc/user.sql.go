// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: user.sql

package db

import (
	"context"
)

const createUser = `-- name: CreateUser :one

INSERT INTO users (
  username,
  password_plaintext
) VALUES (
  $1, $2
) RETURNING id, username, password_plaintext, status, created_at
`

type CreateUserParams struct {
	Username          string `json:"username"`
	PasswordPlaintext string `json:"password_plaintext"`
}

// db/query/user.sql
func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) {
	row := q.db.QueryRowContext(ctx, createUser, arg.Username, arg.PasswordPlaintext)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Username,
		&i.PasswordPlaintext,
		&i.Status,
		&i.CreatedAt,
	)
	return i, err
}

const getUserByID = `-- name: GetUserByID :one
SELECT id, username, password_plaintext, status, created_at FROM users
WHERE id = $1 LIMIT 1
`

func (q *Queries) GetUserByID(ctx context.Context, id int32) (User, error) {
	row := q.db.QueryRowContext(ctx, getUserByID, id)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Username,
		&i.PasswordPlaintext,
		&i.Status,
		&i.CreatedAt,
	)
	return i, err
}

const getUserByUsername = `-- name: GetUserByUsername :one
SELECT id, username, password_plaintext, status, created_at FROM users
WHERE username = $1 LIMIT 1
`

func (q *Queries) GetUserByUsername(ctx context.Context, username string) (User, error) {
	row := q.db.QueryRowContext(ctx, getUserByUsername, username)
	var i User
	err := row.Scan(
		&i.ID,
		&i.Username,
		&i.PasswordPlaintext,
		&i.Status,
		&i.CreatedAt,
	)
	return i, err
}

const listOfflineUsers = `-- name: ListOfflineUsers :many
SELECT id, username FROM users
WHERE status = 'offline'
ORDER BY username
`

type ListOfflineUsersRow struct {
	ID       int32  `json:"id"`
	Username string `json:"username"`
}

func (q *Queries) ListOfflineUsers(ctx context.Context) ([]ListOfflineUsersRow, error) {
	rows, err := q.db.QueryContext(ctx, listOfflineUsers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListOfflineUsersRow{}
	for rows.Next() {
		var i ListOfflineUsersRow
		if err := rows.Scan(&i.ID, &i.Username); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listOnlineUsers = `-- name: ListOnlineUsers :many
SELECT id, username FROM users
WHERE status = 'online'
ORDER BY username
`

type ListOnlineUsersRow struct {
	ID       int32  `json:"id"`
	Username string `json:"username"`
}

func (q *Queries) ListOnlineUsers(ctx context.Context) ([]ListOnlineUsersRow, error) {
	rows, err := q.db.QueryContext(ctx, listOnlineUsers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListOnlineUsersRow{}
	for rows.Next() {
		var i ListOnlineUsersRow
		if err := rows.Scan(&i.ID, &i.Username); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateUserStatus = `-- name: UpdateUserStatus :exec
UPDATE users
SET status = $2
WHERE id = $1
`

type UpdateUserStatusParams struct {
	ID     int32  `json:"id"`
	Status string `json:"status"`
}

func (q *Queries) UpdateUserStatus(ctx context.Context, arg UpdateUserStatusParams) error {
	_, err := q.db.ExecContext(ctx, updateUserStatus, arg.ID, arg.Status)
	return err
}
