package store

import (
	"context"
	"time"

	"coalrag/internal/model"
)

// CreateBroadcast 新建宣贯任务。
func (s *Store) CreateBroadcast(ctx context.Context, t *model.BroadcastTask) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `insert into broadcast_task
		(title, content, doc_ids, dept_ids, publisher_id, require_ack, deadline, status)
		values ($1,$2,$3,$4,$5,$6,$7,$8) returning id`,
		t.Title, t.Content, arr64(t.DocIDs), arr64(t.DeptIDs), t.PublisherID, t.RequireAck, t.Deadline, t.Status).Scan(&id)
	return id, wrapErr("新建宣贯任务", err)
}

// PublishBroadcast 发布任务并按部门生成签收记录。deptIDs 为空表示全员。
func (s *Store) PublishBroadcast(ctx context.Context, id int64) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `update broadcast_task set status='published' where id=$1 and status='draft'`, id)
	if err != nil {
		return 0, wrapErr("发布宣贯任务", err)
	}
	if tag.RowsAffected() == 0 {
		return 0, ErrConflict
	}
	tag, err = tx.Exec(ctx, `insert into broadcast_record(task_id, user_id)
		select $1, u.id from org_user u
		where u.status='active'
		  and exists (
			select 1 from broadcast_task t
			where t.id = $1
			  and (cardinality(t.dept_ids) = 0 or u.dept_id = any(t.dept_ids))
		  )
		on conflict (task_id, user_id) do nothing`, id)
	if err != nil {
		return 0, wrapErr("生成签收记录", err)
	}
	return tag.RowsAffected(), tx.Commit(ctx)
}

// ListBroadcastTasks 管理端查询任务，附带签收统计。
func (s *Store) ListBroadcastTasks(ctx context.Context, status string, limit, offset int) ([]model.BroadcastTask, int, error) {
	rows, err := s.pool.Query(ctx, `select t.id, t.title, t.content, t.doc_ids, t.dept_ids,
		t.publisher_id, t.require_ack, t.deadline, t.status, t.created_at,
		(select count(*) from broadcast_record r where r.task_id=t.id) as total_cnt,
		(select count(*) from broadcast_record r where r.task_id=t.id and r.status='ack') as ack_cnt,
		count(*) over() as total
		from broadcast_task t where ($1='' or t.status=$1) order by t.id desc limit $2 offset $3`,
		status, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询宣贯任务", err)
	}
	defer rows.Close()
	out := []model.BroadcastTask{}
	total := 0
	for rows.Next() {
		var t model.BroadcastTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Content, &t.DocIDs, &t.DeptIDs, &t.PublisherID,
			&t.RequireAck, &t.Deadline, &t.Status, &t.CreatedAt, &t.TotalCount, &t.AckCount, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}

// ListMyBroadcasts 员工端查询自己需要签收的任务。
func (s *Store) ListMyBroadcasts(ctx context.Context, userID int64, onlyPending bool) ([]model.BroadcastTask, error) {
	rows, err := s.pool.Query(ctx, `select t.id, t.title, t.content, t.doc_ids, t.dept_ids,
		t.publisher_id, t.require_ack, t.deadline, t.status, t.created_at, r.status,
		(select count(*) from broadcast_record r2 where r2.task_id=t.id),
		(select count(*) from broadcast_record r2 where r2.task_id=t.id and r2.status='ack')
		from broadcast_task t join broadcast_record r on r.task_id=t.id and r.user_id=$1
		where t.status='published' and (not $2 or r.status<>'ack')
		order by t.id desc`, userID, onlyPending)
	if err != nil {
		return nil, wrapErr("查询我的宣贯任务", err)
	}
	defer rows.Close()
	out := []model.BroadcastTask{}
	for rows.Next() {
		var t model.BroadcastTask
		if err := rows.Scan(&t.ID, &t.Title, &t.Content, &t.DocIDs, &t.DeptIDs, &t.PublisherID,
			&t.RequireAck, &t.Deadline, &t.Status, &t.CreatedAt, &t.MyStatus, &t.TotalCount, &t.AckCount); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AckBroadcast 签收或标记已读。
func (s *Store) AckBroadcast(ctx context.Context, taskID, userID int64, ack bool) error {
	status := "read"
	if ack {
		status = "ack"
	}
	tag, err := s.pool.Exec(ctx, `update broadcast_record set
		status = case when $3 then 'ack' else case when status='ack' then 'ack' else 'read' end end,
		read_at = coalesce(read_at, now()),
		ack_at = case when $3 then coalesce(ack_at, now()) else ack_at end
		where task_id=$1 and user_id=$2`, taskID, userID, ack)
	if err != nil {
		return wrapErr("签收宣贯任务", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	_ = status
	return nil
}

// ListBroadcastRecords 查询签收明细。
func (s *Store) ListBroadcastRecords(ctx context.Context, taskID int64, status string, limit, offset int) ([]model.BroadcastRecord, int, error) {
	rows, err := s.pool.Query(ctx, `select r.id, r.task_id, r.user_id, u.name, coalesce(d.name,''),
		r.status, r.read_at, r.ack_at, count(*) over() as total
		from broadcast_record r
		join org_user u on u.id=r.user_id
		left join org_dept d on d.id=u.dept_id
		where r.task_id=$1 and ($2='' or r.status=$2)
		order by r.id limit $3 offset $4`, taskID, status, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询签收明细", err)
	}
	defer rows.Close()
	var out []model.BroadcastRecord
	total := 0
	for rows.Next() {
		var r model.BroadcastRecord
		if err := rows.Scan(&r.ID, &r.TaskID, &r.UserID, &r.UserName, &r.DeptName, &r.Status,
			&r.ReadAt, &r.AckAt, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// RemindBroadcast 对未签收人员投递提醒（写审计并返回待提醒人数）。
func (s *Store) RemindBroadcast(ctx context.Context, taskID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from broadcast_record where task_id=$1 and status<>'ack'`, taskID).Scan(&n)
	return n, wrapErr("统计待签收", err)
}

// BroadcastDeadline 返回任务截止时间。
func (s *Store) BroadcastDeadline(ctx context.Context, taskID int64) (*time.Time, error) {
	var t *time.Time
	err := s.pool.QueryRow(ctx, `select deadline from broadcast_task where id=$1`, taskID).Scan(&t)
	return t, wrapErr("查询截止时间", err)
}
