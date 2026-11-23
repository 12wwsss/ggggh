package store

import (
	"context"

	"coalrag/internal/model"
)

// IssueCateIDByName 按名称取分类 id；不存在返回 0。
func (s *Store) IssueCateIDByName(ctx context.Context, name string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `select id from issue_cate where name=$1`, name).Scan(&id)
	if isNoRows(err) {
		return 0, nil
	}
	if err != nil {
		return 0, wrapErr("查询问题分类", err)
	}
	return id, nil
}

// ListIssueCates 列出问题分类，供上报表单与筛选使用。
func (s *Store) ListIssueCates(ctx context.Context) ([]model.IssueCate, error) {
	rows, err := s.pool.Query(ctx, `select id, name, parent_id, sort from issue_cate order by sort, id`)
	if err != nil {
		return nil, wrapErr("查询问题分类", err)
	}
	defer rows.Close()

	out := []model.IssueCate{}
	for rows.Next() {
		var c model.IssueCate
		if err := rows.Scan(&c.ID, &c.Name, &c.ParentID, &c.Sort); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
