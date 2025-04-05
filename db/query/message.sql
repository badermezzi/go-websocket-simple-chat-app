-- name: CreateMessage :one
INSERT INTO messages (
  sender_id,
  receiver_id,
  content
) VALUES (
  $1, $2, $3
) RETURNING *;

-- name: GetMessagesBetweenUsers :many
SELECT * FROM messages
WHERE (sender_id = $1 AND receiver_id = $2)
   OR (sender_id = $2 AND receiver_id = $1)
ORDER BY created_at DESC -- Order by newest first for pagination
LIMIT $3 -- Page size
OFFSET $4; -- Offset for pagination