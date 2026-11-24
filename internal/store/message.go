package store

import (
	"context"

	"coalrag/internal/model"
)

// Message 站内消息。
type Message struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	RefType   string `json:"ref_type"`
	RefID     *int64 `json:"ref_id"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"created_at"`
}

// CreateMessage 写入一条消息。
func (s *Store) CreateMessage(ctx context.Context, userID int64, kind, title, body, refType string, refID *int64) error {
	if userID <= 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `insert into message(user_id, kind, title, body, ref_type, ref_id)
		values ($1,$2,$3,$4,nullif($5,''),$6)`, userID, kind, title, body, refType, refID)
	return wrapErr("写入消息", err)
}

// ListMessages 我的消息（按时间倒序）。
func (s *Store) ListMessages(ctx context.Context, userID int64, limit, offset int) ([]Message, int, error) {
	rows, err := s.pool.Query(ctx, `select id, kind, title, body, coalesce(ref_type,''), ref_id, read,
		to_char(created_at,'MM-DD HH24:MI') from message where user_id=$1
		order by id desc limit $2 offset $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询消息", err)
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Kind, &m.Title, &m.Body, &m.RefType, &m.RefID, &m.Read, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	var total int
	err = s.pool.QueryRow(ctx, `select count(*) from message where user_id=$1`, userID).Scan(&total)
	return out, total, wrapErr("统计消息", err)
}

// UnreadMessageCount 未读数。
func (s *Store) UnreadMessageCount(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from message where user_id=$1 and not read`, userID).Scan(&n)
	return n, wrapErr("统计未读消息", err)
}

// MarkMessageRead 标记单条已读。
func (s *Store) MarkMessageRead(ctx context.Context, userID, id int64) error {
	_, err := s.pool.Exec(ctx, `update message set read=true where id=$1 and user_id=$2`, id, userID)
	return wrapErr("标记已读", err)
}

// MarkAllMessagesRead 全部已读。
func (s *Store) MarkAllMessagesRead(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `update message set read=true where user_id=$1 and not read`, userID)
	return wrapErr("全部已读", err)
}

var _ = model.Issue{}
