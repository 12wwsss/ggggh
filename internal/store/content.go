package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ContentItem 是安全文化 / 图文 / 宣贯类内容条目。
// payload 存页面直接需要的对象（body、summary、date…），避免为每种内容建表。
type ContentItem struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Cat       string         `json:"cat"`
	Title     string         `json:"title"`
	Sort      int            `json:"sort"`
	Status    string         `json:"status"`
	Payload   map[string]any `json:"payload"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// normalizeContentBody 统一把 body 规范成段落数组。
// 历史数据里 culture 类的 body 是一整段字符串，小程序按数组渲染，
// 会退化成一个字一段；读出口径统一成 []string 后两端都能正常显示。
func normalizeContentBody(m map[string]any) {
	if m == nil {
		return
	}
	switch v := m["body"].(type) {
	case string:
		paras := []string{}
		for _, line := range strings.Split(strings.ReplaceAll(v, "\r\n", "\n"), "\n") {
			if s := strings.TrimSpace(line); s != "" {
				paras = append(paras, s)
			}
		}
		m["body"] = paras
	case []any:
		paras := make([]string, 0, len(v))
		for _, it := range v {
			if s, ok := it.(string); ok && strings.TrimSpace(s) != "" {
				paras = append(paras, strings.TrimSpace(s))
			}
		}
		m["body"] = paras
	}
}

// ListContentItems 按类型/分类列出已发布内容，payload 原样返回给小程序与 H5。
func (s *Store) ListContentItems(ctx context.Context, kind, cat string) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `select payload from content_item
		where kind=$1 and status='published' and ($2='' or cat=$2)
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
		normalizeContentBody(m)
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListContent 管理端查询内容，status 为空表示含草稿与下架。
func (s *Store) ListContent(ctx context.Context, kind, status string, limit int) ([]ContentItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `select id, kind, coalesce(cat,''), title, sort, status, payload, updated_at
		from content_item
		where ($1='' or kind=$1) and ($2='' or status=$2)
		order by kind, cat, sort, id limit $3`, kind, status, limit)
	if err != nil {
		return nil, wrapErr("查询内容列表", err)
	}
	defer rows.Close()
	out := []ContentItem{}
	for rows.Next() {
		var it ContentItem
		var raw []byte
		if err := rows.Scan(&it.ID, &it.Kind, &it.Cat, &it.Title, &it.Sort, &it.Status, &raw, &it.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &it.Payload)
		normalizeContentBody(it.Payload)
		out = append(out, it)
	}
	return out, rows.Err()
}

// GetContentItem 取单条内容（含草稿，管理端预览用）。
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
	normalizeContentBody(m)
	return m, nil
}

// GetPublishedContent 取单条已发布内容，员工端用。
func (s *Store) GetPublishedContent(ctx context.Context, id string) (map[string]any, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `select payload from content_item where id=$1 and status='published'`, id).Scan(&raw)
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
	normalizeContentBody(m)
	return m, nil
}

// UpsertContent 新增或修改内容；id 为空时生成。
func (s *Store) UpsertContent(ctx context.Context, it *ContentItem, userID int64) (string, error) {
	if strings.TrimSpace(it.ID) == "" {
		it.ID = "c-" + uuid.NewString()[:12]
	}
	if it.Payload == nil {
		it.Payload = map[string]any{}
	}
	it.Payload["id"] = it.ID
	if it.Cat != "" {
		it.Payload["cat"] = it.Cat
	}
	it.Payload["title"] = it.Title
	raw, err := json.Marshal(it.Payload)
	if err != nil {
		return "", err
	}
	_, err = s.pool.Exec(ctx, `insert into content_item(id, kind, cat, title, sort, status, payload, updated_by)
		values ($1,$2,$3,$4,$5,$6,$7,$8)
		on conflict (id) do update set kind=excluded.kind, cat=excluded.cat, title=excluded.title,
			sort=excluded.sort, status=excluded.status, payload=excluded.payload,
			updated_by=excluded.updated_by, updated_at=now()`,
		it.ID, it.Kind, it.Cat, it.Title, it.Sort, it.Status, raw, userID)
	if err != nil {
		return "", wrapErr("保存内容", err)
	}
	return it.ID, nil
}

// DeleteContent 删除内容。
func (s *Store) DeleteContent(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `delete from content_item where id=$1`, id)
	if err != nil {
		return wrapErr("删除内容", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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
