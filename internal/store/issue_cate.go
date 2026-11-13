package store

import (
	"context"

	"coalrag/internal/model"
)

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
