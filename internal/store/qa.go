package store

import (
	"context"
	"encoding/json"
	"time"

	"coalrag/internal/model"
)

// InsertQALog 写入问答留痕。
func (s *Store) InsertQALog(ctx context.Context, l *model.QALog) (int64, error) {
	cites, err := json.Marshal(l.Citations)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.pool.QueryRow(ctx, `insert into qa_log
		(user_id, dept_id, question, answer, citations, answered, top_score, hit_count, latency_ms, model, ip)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,nullif($11,'')) returning id`,
		l.UserID, l.DeptID, l.Question, l.Answer, cites, l.Answered, l.TopScore, l.HitCount,
		l.LatencyMS, l.Model, l.IP).Scan(&id)
	return id, wrapErr("写入问答记录", err)
}

// ListQALogs 按数据范围查询问答记录。scope: all|dept|self。
func (s *Store) ListQALogs(ctx context.Context, scope string, userID, deptID int64, answered *bool, limit, offset int) ([]model.QALog, int, error) {
	var dept *int64
	if scope == "dept" {
		dept = &deptID
	}
	var uid *int64
	if scope == "self" {
		uid = &userID
	}
	q := `select q.id, q.user_id, coalesce(u.name,''), q.dept_id, q.question, q.answer, q.citations,
		q.answered, q.top_score, q.hit_count, q.latency_ms, coalesce(q.model,''),
		coalesce(q.feedback,''), coalesce(q.ip,''), q.created_at, count(*) over() as total
		from qa_log q left join org_user u on u.id=q.user_id
		where ($1::bigint is null or q.user_id=$1)
		  and ($2::bigint is null or q.dept_id=$2)
		  and ($3::boolean is null or q.answered=$3)
		order by q.id desc limit $4 offset $5`
	rows, err := s.pool.Query(ctx, q, uid, dept, answered, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询问答记录", err)
	}
	defer rows.Close()
	var out []model.QALog
	total := 0
	for rows.Next() {
		var l model.QALog
		var cites []byte
		if err := rows.Scan(&l.ID, &l.UserID, &l.UserName, &l.DeptID, &l.Question, &l.Answer, &cites,
			&l.Answered, &l.TopScore, &l.HitCount, &l.LatencyMS, &l.Model, &l.Feedback, &l.IP,
			&l.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(cites, &l.Citations)
		out = append(out, l)
	}
	return out, total, rows.Err()
}

// GetQALog 取单条。
func (s *Store) GetQALog(ctx context.Context, id int64) (*model.QALog, error) {
	var l model.QALog
	var cites []byte
	err := s.pool.QueryRow(ctx, `select q.id, q.user_id, coalesce(u.name,''), q.dept_id, q.question,
		q.answer, q.citations, q.answered, q.top_score, q.hit_count, q.latency_ms,
		coalesce(q.model,''), coalesce(q.feedback,''), coalesce(q.ip,''), q.created_at
		from qa_log q left join org_user u on u.id=q.user_id where q.id=$1`, id).
		Scan(&l.ID, &l.UserID, &l.UserName, &l.DeptID, &l.Question, &l.Answer, &cites,
			&l.Answered, &l.TopScore, &l.HitCount, &l.LatencyMS, &l.Model, &l.Feedback, &l.IP, &l.CreatedAt)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询问答记录", err)
	}
	_ = json.Unmarshal(cites, &l.Citations)
	return &l, nil
}

// SetQALogFeedback 记录用户反馈。
func (s *Store) SetQALogFeedback(ctx context.Context, id int64, feedback string) error {
	tag, err := s.pool.Exec(ctx, `update qa_log set feedback=$2 where id=$1`, id, feedback)
	if err != nil {
		return wrapErr("写入反馈", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- 审计 ----------

// InsertAudit 写入审计日志。
func (s *Store) InsertAudit(ctx context.Context, l *model.AuditLog) error {
	detail, _ := json.Marshal(l.Detail)
	_, err := s.pool.Exec(ctx, `insert into audit_log
		(user_id, username, action, resource, resource_id, detail, ip, ua)
		values ($1,$2,$3,$4,nullif($5,''),$6,$7,$8)`,
		l.UserID, l.Username, l.Action, l.Resource, l.ResourceID, detail, l.IP, l.UA)
	return wrapErr("写入审计日志", err)
}

// ListAudit 查询审计日志。
func (s *Store) ListAudit(ctx context.Context, resource, action string, limit, offset int) ([]model.AuditLog, int, error) {
	rows, err := s.pool.Query(ctx, `select id, user_id, coalesce(username,''), action, resource,
		coalesce(resource_id,''), detail, coalesce(ip,''), coalesce(ua,''), created_at, count(*) over() as total
		from audit_log where ($1='' or resource=$1) and ($2='' or action=$2)
		order by id desc limit $3 offset $4`, resource, action, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询审计日志", err)
	}
	defer rows.Close()
	var out []model.AuditLog
	total := 0
	for rows.Next() {
		var l model.AuditLog
		var detail []byte
		if err := rows.Scan(&l.ID, &l.UserID, &l.Username, &l.Action, &l.Resource, &l.ResourceID,
			&detail, &l.IP, &l.UA, &l.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		l.Detail = json.RawMessage(detail)
		out = append(out, l)
	}
	return out, total, rows.Err()
}

// ---------- 元数据 ----------

// GetMeta 读元数据。
func (s *Store) GetMeta(ctx context.Context, key string) (string, error) {
	var v string
	err := s.pool.QueryRow(ctx, `select value from system_meta where key=$1`, key).Scan(&v)
	if isNoRows(err) {
		return "", ErrNotFound
	}
	return v, wrapErr("读取元数据", err)
}

// PutMeta 写元数据。
func (s *Store) PutMeta(ctx context.Context, key, value string) error {
	_, err := s.pool.Exec(ctx, `insert into system_meta(key,value) values($1,$2)
		on conflict (key) do update set value=excluded.value, updated_at=now()`, key, value)
	return wrapErr("写入元数据", err)
}

// ---------- 任务 ----------

// Enqueue 投递后台任务。
func (s *Store) Enqueue(ctx context.Context, kind string, payload any, delay time.Duration) (int64, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.pool.QueryRow(ctx, `insert into task(kind, payload, run_after)
		values ($1,$2, now() + $3::interval) returning id`,
		kind, b, durationExpr(delay)).Scan(&id)
	return id, wrapErr("投递任务", err)
}

func durationExpr(d time.Duration) string {
	if d <= 0 {
		return "0 seconds"
	}
	return d.String()
}

// ClaimTasks 领取待执行任务（FOR UPDATE SKIP LOCKED，支持多进程）。
func (s *Store) ClaimTasks(ctx context.Context, limit int) ([]model.Task, error) {
	rows, err := s.pool.Query(ctx, `update task set status='running', attempts=attempts+1,
		locked_at=now(), updated_at=now()
		where id in (select id from task where status='pending' and run_after<=now()
			order by id limit $1 for update skip locked)
		returning id, kind, payload, status, attempts, max_attempts, coalesce(last_error,''), run_after, created_at`, limit)
	if err != nil {
		return nil, wrapErr("领取任务", err)
	}
	defer rows.Close()
	var out []model.Task
	for rows.Next() {
		var t model.Task
		var payload []byte
		if err := rows.Scan(&t.ID, &t.Kind, &payload, &t.Status, &t.Attempts, &t.MaxAttempts,
			&t.LastError, &t.RunAfter, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Payload = json.RawMessage(payload)
		out = append(out, t)
	}
	return out, rows.Err()
}

// FinishTask 标记任务成功。
func (s *Store) FinishTask(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `update task set status='succeeded', last_error=null,
		locked_at=null, updated_at=now() where id=$1`, id)
	return wrapErr("完成任务", err)
}

// FailTask 标记任务失败：未达上限则退避重试。
func (s *Store) FailTask(ctx context.Context, id int64, msg string, backoff time.Duration) error {
	_, err := s.pool.Exec(ctx, `update task set
		status = case when attempts >= max_attempts then 'failed' else 'pending' end,
		last_error = $2,
		run_after = case when attempts >= max_attempts then run_after else now() + $3::interval end,
		locked_at = null, updated_at = now() where id=$1`, id, msg, durationExpr(backoff))
	return wrapErr("更新任务状态", err)
}

// RetryTask 人工重试失败任务。
func (s *Store) RetryTask(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `update task set status='pending', attempts=0, last_error=null,
		run_after=now(), updated_at=now() where id=$1 and status in ('failed','succeeded')`, id)
	if err != nil {
		return wrapErr("重试任务", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RecoverStaleTasks 服务重启时把僵死任务放回队列。
func (s *Store) RecoverStaleTasks(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx, `update task set status='pending', locked_at=null, updated_at=now()
		where status='running' and locked_at < now() - $1::interval`, durationExpr(olderThan))
	if err != nil {
		return 0, wrapErr("恢复僵死任务", err)
	}
	return tag.RowsAffected(), nil
}

// ListTasks 查询任务。
func (s *Store) ListTasks(ctx context.Context, status string, limit, offset int) ([]model.Task, int, error) {
	rows, err := s.pool.Query(ctx, `select id, kind, payload, status, attempts, max_attempts,
		coalesce(last_error,''), run_after, created_at, count(*) over() as total
		from task where ($1='' or status=$1) order by id desc limit $2 offset $3`, status, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询任务", err)
	}
	defer rows.Close()
	var out []model.Task
	total := 0
	for rows.Next() {
		var t model.Task
		var payload []byte
		if err := rows.Scan(&t.ID, &t.Kind, &payload, &t.Status, &t.Attempts, &t.MaxAttempts,
			&t.LastError, &t.RunAfter, &t.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		t.Payload = json.RawMessage(payload)
		out = append(out, t)
	}
	return out, total, rows.Err()
}
