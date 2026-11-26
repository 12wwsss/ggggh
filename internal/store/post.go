package store

import (
	"context"

	"coalrag/internal/model"
)

// ListPosts 岗位列表。
func (s *Store) ListPosts(ctx context.Context, activeOnly bool) ([]model.Post, error) {
	rows, err := s.pool.Query(ctx, `select id, name, sort, status from org_post
		where (not $1 or status='active') order by sort, id`, activeOnly)
	if err != nil {
		return nil, wrapErr("查询岗位", err)
	}
	defer rows.Close()
	out := []model.Post{}
	for rows.Next() {
		var p model.Post
		if err := rows.Scan(&p.ID, &p.Name, &p.Sort, &p.Status); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PostExists 岗位是否存在（启用状态）。
func (s *Store) PostExists(ctx context.Context, name string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists(select 1 from org_post where name=$1 and status='active')`, name).Scan(&ok)
	return ok, wrapErr("查询岗位", err)
}

// CreatePost 新增岗位。
func (s *Store) CreatePost(ctx context.Context, p *model.Post) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `insert into org_post(name, sort, status) values ($1,$2,$3) returning id`,
		p.Name, p.Sort, p.Status).Scan(&id)
	return id, wrapErr("新增岗位", err)
}

// UpdatePost 修改岗位；改名时级联更新账号、学习条目、风险点里的岗位名称。
func (s *Store) UpdatePost(ctx context.Context, p *model.Post) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var old string
	if err := tx.QueryRow(ctx, `select name from org_post where id=$1`, p.ID).Scan(&old); err != nil {
		return wrapErr("查询岗位", err)
	}
	if _, err := tx.Exec(ctx, `update org_post set name=$2, sort=$3, status=$4 where id=$1`,
		p.ID, p.Name, p.Sort, p.Status); err != nil {
		return wrapErr("更新岗位", err)
	}
	if p.Name != old {
		if _, err := tx.Exec(ctx, `update org_user set post=$2, updated_at=now() where post=$1`, old, p.Name); err != nil {
			return wrapErr("同步账号岗位", err)
		}
		if _, err := tx.Exec(ctx, `update learn_item set post=$2, updated_at=now() where post=$1`, old, p.Name); err != nil {
			return wrapErr("同步学习条目岗位", err)
		}
		if _, err := tx.Exec(ctx, `update risk_point set post=$2 where post=$1`, old, p.Name); err != nil {
			return wrapErr("同步风险点岗位", err)
		}
	}
	return tx.Commit(ctx)
}

// DeletePost 删除岗位；仍被账号使用时拒绝。
func (s *Store) DeletePost(ctx context.Context, id int64) error {
	var name string
	if err := s.pool.QueryRow(ctx, `select name from org_post where id=$1`, id).Scan(&name); err != nil {
		return wrapErr("查询岗位", err)
	}
	var used int
	if err := s.pool.QueryRow(ctx, `select count(*) from org_user where post=$1`, name).Scan(&used); err != nil {
		return wrapErr("统计岗位", err)
	}
	if used > 0 {
		return ErrConflict
	}
	_, err := s.pool.Exec(ctx, `delete from org_post where id=$1`, id)
	return wrapErr("删除岗位", err)
}
