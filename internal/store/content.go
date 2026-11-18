package store

import (
	"context"
	"encoding/json"
)

// ListContentItems 按类型/分类列出内容，payload 原样返回给小程序。
func (s *Store) ListContentItems(ctx context.Context, kind, cat string) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `select payload from content_item
		where kind=$1 and ($2='' or cat=$2)
		order by sort, id`, kind, cat)
	if err != nil {
		return nil, wrapErr("查询内容", err)
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetContentItem 取单条内容。
func (s *Store) GetContentItem(ctx context.Context, id string) (map[string]any, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `select payload from content_item where id=$1`, id).Scan(&raw)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询内容", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// DistinctPosts 返回在册人员中出现的岗位，用于小程序岗位选项。
func (s *Store) DistinctPosts(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `select distinct post from org_user
		where coalesce(post,'') <> '' order by 1`)
	if err != nil {
		return nil, wrapErr("查询岗位", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListDeptNames 返回部门名称，用于小程序区队选项。
func (s *Store) ListDeptNames(ctx context.Context) ([]string, error) {
	depts, err := s.ListDepts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(depts))
	for _, d := range depts {
		out = append(out, d.Name)
	}
	return out, nil
}
