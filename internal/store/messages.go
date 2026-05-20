package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/juliancanalez/pane/internal/messages"
)

type MessageStore struct {
	db *sql.DB
}

func NewMessageStore(db *sql.DB) MessageStore {
	return MessageStore{db: db}
}

func (s MessageStore) Save(ctx context.Context, value messages.Message) error {
	var deliveredAt any
	if value.DeliveredAt != nil {
		deliveredAt = *value.DeliveredAt
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO messages (message_id, thread_id, from_session, to_session, body, state, created_at, delivered_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
`, value.ID, value.ThreadID, value.FromSession, value.ToSession, value.Body, string(value.State), value.CreatedAt, deliveredAt)
	return err
}

func (s MessageStore) FindByID(ctx context.Context, messageID string) (messages.Message, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT message_id, thread_id, from_session, to_session, body, state, created_at, delivered_at
FROM messages
WHERE message_id = ?
`, messageID)
	return scanMessage(row)
}

func (s MessageStore) ListQueuedForSession(ctx context.Context, sessionID string) ([]messages.Message, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT message_id, thread_id, from_session, to_session, body, state, created_at, delivered_at
FROM messages
WHERE state = 'queued'
  AND (to_session = ? OR to_session = '*')
ORDER BY created_at ASC
`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []messages.Message
	for rows.Next() {
		value, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s MessageStore) MarkDelivered(ctx context.Context, messageIDs []string, deliveredAt int64) error {
	for _, id := range messageIDs {
		if _, err := s.db.ExecContext(ctx, `
UPDATE messages
SET state = 'delivered', delivered_at = ?
WHERE message_id = ?
`, deliveredAt, id); err != nil {
			return err
		}
	}
	return nil
}

type messageScanner interface {
	Scan(dest ...any) error
}

func scanMessage(row messageScanner) (messages.Message, error) {
	var value messages.Message
	var state string
	var deliveredAt sql.NullInt64
	err := row.Scan(&value.ID, &value.ThreadID, &value.FromSession, &value.ToSession, &value.Body, &state, &value.CreatedAt, &deliveredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return messages.Message{}, ErrNotFound
	}
	if err != nil {
		return messages.Message{}, err
	}
	value.State = messages.State(state)
	if deliveredAt.Valid {
		value.DeliveredAt = &deliveredAt.Int64
	}
	return value, nil
}
