package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/messenger/chat-service/internal/domain"
)

func (r *Repository) Create(ctx context.Context, chat *domain.Chat) error {
	q := `
		INSERT INTO chats (id, type, title, avatar_url, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	db := r.getQueryExec(ctx)
	_, err := db.Exec(ctx, q, chat.ID, chat.Type, chat.Title, chat.AvatarURL, chat.CreatedBy, chat.CreatedAt, chat.UpdatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Chat, error) {
	q := `
		SELECT id, type, title, avatar_url, created_by, last_message_at, deleted_at, created_at, updated_at,
		       COALESCE((SELECT COUNT(*) FROM chat_members WHERE chat_id = chats.id AND left_at IS NULL), 0) as members_count
		FROM chats
		WHERE id = $1 AND deleted_at IS NULL
	`
	row := r.getQueryExec(ctx).QueryRow(ctx, q, id)

	var chat domain.Chat
	err := row.Scan(
		&chat.ID, &chat.Type, &chat.Title, &chat.AvatarURL,
		&chat.CreatedBy, &chat.LastMessageAt, &chat.DeletedAt,
		&chat.CreatedAt, &chat.UpdatedAt, &chat.MembersCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrChatNotFound
		}
		return nil, err
	}
	return &chat, nil
}

func (r *Repository) Update(ctx context.Context, chat *domain.Chat) error {
	q := `
		UPDATE chats
		SET title = $1, avatar_url = $2, updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL
	`
	db := r.getQueryExec(ctx)
	cmd, err := db.Exec(ctx, q, chat.Title, chat.AvatarURL, chat.ID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrChatNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	q := `
		UPDATE chats
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
	db := r.getQueryExec(ctx)
	cmd, err := db.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrChatNotFound
	}
	return nil
}

func (r *Repository) ListByUserID(ctx context.Context, userID uuid.UUID, limit int, offset int) ([]*domain.Chat, error) {
	// Join with chat_members to find chats for user
	// Sort by COALESCE(last_message_at, created_at) DESC
	q := `
		SELECT c.id, c.type, c.title, c.avatar_url, c.created_by, c.last_message_at, c.deleted_at, c.created_at, c.updated_at,
		       COALESCE((SELECT COUNT(*) FROM chat_members WHERE chat_id = c.id AND left_at IS NULL), 0) as members_count
		FROM chats c
		JOIN chat_members cm ON cm.chat_id = c.id
		WHERE cm.user_id = $1 AND cm.left_at IS NULL AND c.deleted_at IS NULL
		ORDER BY COALESCE(c.last_message_at, c.created_at) DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.getQueryExec(ctx).Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []*domain.Chat
	for rows.Next() {
		var c domain.Chat
		err := rows.Scan(
			&c.ID, &c.Type, &c.Title, &c.AvatarURL,
			&c.CreatedBy, &c.LastMessageAt, &c.DeletedAt,
			&c.CreatedAt, &c.UpdatedAt, &c.MembersCount,
		)
		if err != nil {
			return nil, err
		}
		chats = append(chats, &c)
	}
	return chats, nil
}

func (r *Repository) GetDirectChat(ctx context.Context, userA, userB uuid.UUID) (*domain.Chat, error) {
	// Ensure order for index usage
	// user_id_a < user_id_b invarian from DB schema
	var u1, u2 uuid.UUID
	if compareUUID(userA, userB) < 0 {
		u1, u2 = userA, userB
	} else {
		u1, u2 = userB, userA
	}

	q := `
		SELECT c.id, c.type, c.title, c.avatar_url, c.created_by, c.last_message_at, c.deleted_at, c.created_at, c.updated_at,
		       COALESCE((SELECT COUNT(*) FROM chat_members WHERE chat_id = c.id AND left_at IS NULL), 0) as members_count
		FROM direct_chat_index dci
		JOIN chats c ON c.id = dci.chat_id
		WHERE dci.user_id_a = $1 AND dci.user_id_b = $2 AND c.deleted_at IS NULL
	`
	row := r.getQueryExec(ctx).QueryRow(ctx, q, u1, u2)

	var chat domain.Chat
	err := row.Scan(
		&chat.ID, &chat.Type, &chat.Title, &chat.AvatarURL,
		&chat.CreatedBy, &chat.LastMessageAt, &chat.DeletedAt,
		&chat.CreatedAt, &chat.UpdatedAt, &chat.MembersCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrChatNotFound
		}
		return nil, err
	}
	return &chat, nil
}

func (r *Repository) SaveDirectChatIndex(ctx context.Context, userA, userB, chatID uuid.UUID) error {
	var u1, u2 uuid.UUID
	if compareUUID(userA, userB) < 0 {
		u1, u2 = userA, userB
	} else {
		u1, u2 = userB, userA
	}

	q := `INSERT INTO direct_chat_index (user_id_a, user_id_b, chat_id) VALUES ($1, $2, $3)`
	_, err := r.getQueryExec(ctx).Exec(ctx, q, u1, u2, chatID)
	return err
}

func compareUUID(u1, u2 uuid.UUID) int {
	for i := 0; i < 16; i++ {
		if u1[i] < u2[i] {
			return -1
		}
		if u1[i] > u2[i] {
			return 1
		}
	}
	return 0
}

func (r *Repository) UpdateLastMessageAt(ctx context.Context, chatID uuid.UUID, timestamp time.Time) error {
	q := `
		UPDATE chats
		SET last_message_at = $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`
	// Simple update, fire and forget logic often?
	// But using outbox here? No, message service already published event.
	// We consume it and update view.
	_, err := r.getQueryExec(ctx).Exec(ctx, q, timestamp, chatID)
	return err
}

func (r *Repository) UpsertMetadata(ctx context.Context, meta *domain.ChatMetadata) error {
	q := `
		INSERT INTO chat_metadata (chat_id, key, value, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (chat_id, key) DO UPDATE
		SET value = EXCLUDED.value, updated_at = NOW()
	`
	_, err := r.getQueryExec(ctx).Exec(ctx, q, meta.ChatID, meta.Key, meta.Value)
	return err
}

func (r *Repository) GetMetadata(ctx context.Context, chatID uuid.UUID, key string) (*domain.ChatMetadata, error) {
	q := `
		SELECT chat_id, key, value, updated_at
		FROM chat_metadata
		WHERE chat_id = $1 AND key = $2
	`
	row := r.getQueryExec(ctx).QueryRow(ctx, q, chatID, key)

	var m domain.ChatMetadata
	err := row.Scan(&m.ChatID, &m.Key, &m.Value, &m.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMetadataNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (r *Repository) DeleteMetadata(ctx context.Context, chatID uuid.UUID, key string) error {
	q := `DELETE FROM chat_metadata WHERE chat_id = $1 AND key = $2`
	cmd, err := r.getQueryExec(ctx).Exec(ctx, q, chatID, key)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrMetadataNotFound
	}
	return nil
}

func (r *Repository) ListMetadata(ctx context.Context, chatID uuid.UUID) ([]*domain.ChatMetadata, error) {
	q := `SELECT chat_id, key, value, updated_at FROM chat_metadata WHERE chat_id = $1`
	rows, err := r.getQueryExec(ctx).Query(ctx, q, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metas []*domain.ChatMetadata
	for rows.Next() {
		var m domain.ChatMetadata
		err := rows.Scan(&m.ChatID, &m.Key, &m.Value, &m.UpdatedAt)
		if err != nil {
			return nil, err
		}
		metas = append(metas, &m)
	}
	return metas, nil
}

