package store

import (
	"context"

	"coalrag/internal/model"
)

// MediaItem 视频/资料条目。
type MediaItem struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	SizeBytes int64  `json:"size_bytes"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// ListMedia 资料列表（可按分类过滤）。
func (s *Store) ListMedia(ctx context.Context, cat string, limit, offset int) ([]MediaItem, int, error) {
	rows, err := s.pool.Query(ctx, `select id, title, category, type, url, size_bytes,
		to_char(created_at,'YYYY-MM-DD') from media_item
		where status='published' and ($1='' or category=$1)
		order by sort, id desc limit $2 offset $3`, cat, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询资料", err)
	}
	defer rows.Close()
	out := []MediaItem{}
	for rows.Next() {
		var m MediaItem
		if err := rows.Scan(&m.ID, &m.Title, &m.Category, &m.Type, &m.URL, &m.SizeBytes, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, m)
	}
	var total int
	err = s.pool.QueryRow(ctx, `select count(*) from media_item where status='published' and ($1='' or category=$1)`, cat).Scan(&total)
	return out, total, wrapErr("统计资料", err)
}

// MediaCategories 资料分类。
func (s *Store) MediaCategories(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `select distinct category from media_item where status='published' order by 1`)
	if err != nil {
		return nil, wrapErr("查询资料分类", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CreateMedia 新增资料。
func (s *Store) CreateMedia(ctx context.Context, m *MediaItem, userID int64) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `insert into media_item(title, category, type, url, size_bytes, created_by)
		values ($1,$2,$3,$4,$5,$6) returning id`, m.Title, m.Category, m.Type, m.URL, m.SizeBytes, userID).Scan(&id)
	return id, wrapErr("写入资料", err)
}

// DeleteMedia 删除资料并返回 url。
func (s *Store) DeleteMedia(ctx context.Context, id int64) (string, error) {
	var url string
	err := s.pool.QueryRow(ctx, `delete from media_item where id=$1 returning url`, id).Scan(&url)
	if isNoRows(err) {
		return "", ErrNotFound
	}
	return url, wrapErr("删除资料", err)
}

var _ = model.LearnItem{}
