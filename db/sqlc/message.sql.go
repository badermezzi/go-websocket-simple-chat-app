// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: message.sql

package db

import (
	"context"
)

const createMessage = `-- name: CreateMessage :one
INSERT INTO messages (
  sender_id,
  receiver_id,
  content
) VALUES (
  $1, $2, $3
) RETURNING id, sender_id, receiver_id, content, created_at
`

type CreateMessageParams struct {
	SenderID   int32  `json:"sender_id"`
	ReceiverID int32  `json:"receiver_id"`
	Content    string `json:"content"`
}

func (q *Queries) CreateMessage(ctx context.Context, arg CreateMessageParams) (Message, error) {
	row := q.db.QueryRowContext(ctx, createMessage, arg.SenderID, arg.ReceiverID, arg.Content)
	var i Message
	err := row.Scan(
		&i.ID,
		&i.SenderID,
		&i.ReceiverID,
		&i.Content,
		&i.CreatedAt,
	)
	return i, err
}
