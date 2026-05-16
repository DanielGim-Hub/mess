package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/messenger/chat-service/internal/domain"
)

func (r *Repository) Add(ctx context.Context, member *domain.ChatMember) error {
	q := `
		INSERT INTO chat_members (id, chat_id, user_id, role, invited_by, joined_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	// member.ID should be generated if not present, but service usually provides it.
	if member.ID == uuid.Nil {
		member.ID = uuid.New()
	}
	db := r.getQueryExec(ctx)
	_, err := db.Exec(ctx, q, member.ID, member.ChatID, member.UserID, member.Role, member.InvitedBy, member.JoinedAt)
	if err != nil {
		// Handle unique constraint violation if needed
		return err
	}
	return nil
}

func (r *Repository) Remove(ctx context.Context, chatID, userID uuid.UUID) error {
	q := `
		UPDATE chat_members
		SET left_at = NOW()
		WHERE chat_id = $1 AND user_id = $2 AND left_at IS NULL
	`
	db := r.getQueryExec(ctx)
	cmd, err := db.Exec(ctx, q, chatID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrMemberNotFound
	}
	return nil
}

func (r *Repository) UpdateRole(ctx context.Context, chatID, userID uuid.UUID, role domain.MemberRole) error {
	q := `
		UPDATE chat_members
		SET role = $1
		WHERE chat_id = $2 AND user_id = $3 AND left_at IS NULL
	`
	db := r.getQueryExec(ctx)
	cmd, err := db.Exec(ctx, q, role, chatID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrMemberNotFound
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, chatID, userID uuid.UUID) (*domain.ChatMember, error) {
	q := `
		SELECT id, chat_id, user_id, role, invited_by, joined_at, left_at
		FROM chat_members
		WHERE chat_id = $1 AND user_id = $2 AND left_at IS NULL
	`
	row := r.getQueryExec(ctx).QueryRow(ctx, q, chatID, userID)

	var m domain.ChatMember
	err := row.Scan(
		&m.ID, &m.ChatID, &m.UserID, &m.Role, &m.InvitedBy, &m.JoinedAt, &m.LeftAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMemberNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (r *Repository) ListByChatID(ctx context.Context, chatID uuid.UUID) ([]*domain.ChatMember, error) {
	q := `
		SELECT id, chat_id, user_id, role, invited_by, joined_at, left_at
		FROM chat_members
		WHERE chat_id = $1 AND left_at IS NULL
	`
	rows, err := r.getQueryExec(ctx).Query(ctx, q, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*domain.ChatMember
	for rows.Next() {
		var m domain.ChatMember
		err := rows.Scan(
			&m.ID, &m.ChatID, &m.UserID, &m.Role, &m.InvitedBy, &m.JoinedAt, &m.LeftAt,
		)
		if err != nil {
			return nil, err
		}
		members = append(members, &m)
	}
	return members, nil
}

func (r *Repository) Count(ctx context.Context, chatID uuid.UUID) (int64, error) {
	q := `SELECT count(*) FROM chat_members WHERE chat_id = $1 AND left_at IS NULL`
	var count int64
	err := r.getQueryExec(ctx).QueryRow(ctx, q, chatID).Scan(&count)
	return count, err
}

