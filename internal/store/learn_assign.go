package store

import (
	"context"
	"time"
)

// LearnAssignment 学习计划分配。
type LearnAssignment struct {
	ID       int64  `json:"id"`
	ItemID   int64  `json:"item_id"`
	Post     string `json:"post"`
	DeptID   *int64 `json:"dept_id"`
	DeptName string `json:"dept_name"`
	DueAt    string `json:"due_at"`
}

// ListLearnAssignments 某条目的分配列表。
func (s *Store) ListLearnAssignments(ctx context.Context, itemID int64) ([]LearnAssignment, error) {
	rows, err := s.pool.Query(ctx, `select a.id, a.item_id, coalesce(a.post,''), a.dept_id,
		coalesce(d.name,''), coalesce(to_char(a.due_at,'YYYY-MM-DD'),'')
		from learn_assign a left join org_dept d on d.id=a.dept_id
		where ($1=0 or a.item_id=$1) order by a.id desc`, itemID)
	if err != nil {
		return nil, wrapErr("查询学习分配", err)
	}
	defer rows.Close()
	out := []LearnAssignment{}
	for rows.Next() {
		var a LearnAssignment
		if err := rows.Scan(&a.ID, &a.ItemID, &a.Post, &a.DeptID, &a.DeptName, &a.DueAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateLearnAssignment 新增分配。
func (s *Store) CreateLearnAssignment(ctx context.Context, itemID int64, post string, deptID *int64, dueAt *time.Time) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `insert into learn_assign(item_id, post, dept_id, due_at)
		values ($1,$2,$3,$4) returning id`, itemID, post, deptID, dueAt).Scan(&id)
	return id, wrapErr("新增学习分配", err)
}

// DeleteLearnAssignment 删除分配。
func (s *Store) DeleteLearnAssignment(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `delete from learn_assign where id=$1`, id)
	if err != nil {
		return wrapErr("删除学习分配", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LearnAssignmentsForUser 当前用户命中的分配（条目 → 最早截止日）。
func (s *Store) LearnAssignmentsForUser(ctx context.Context, post string, deptID *int64) (map[int64]string, error) {
	rows, err := s.pool.Query(ctx, `select item_id, to_char(min(due_at),'YYYY-MM-DD')
		from learn_assign
		where (post='' or post=$1) and (dept_id is null or dept_id=$2)
		group by item_id`, post, deptID)
	if err != nil {
		return nil, wrapErr("查询学习分配", err)
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var due string
		if err := rows.Scan(&id, &due); err != nil {
			return nil, err
		}
		out[id] = due
	}
	return out, rows.Err()
}

// LearnRequiredStats 必修（含分配）总数与完成数。
func (s *Store) LearnRequiredStats(ctx context.Context, userID int64, post string, deptID *int64) (total, done int, err error) {
	err = s.pool.QueryRow(ctx, `with req as (
			select id from learn_item where status='published' and required
			union
			select item_id from learn_assign where (post='' or post=$2) and (dept_id is null or dept_id=$3)
		)
		select count(*), count(*) filter (where exists (
			select 1 from learn_record r where r.item_id=req.id and r.user_id=$1 and r.status='completed'))
		from req join learn_item li on li.id=req.id where li.status='published'`,
		userID, post, deptID).Scan(&total, &done)
	if err != nil {
		return 0, 0, wrapErr("统计必修学习", err)
	}
	return total, done, nil
}
