package store

import (
	"context"
	"fmt"
	"time"

	"coalrag/internal/model"
)

const sqlIssueCols = `i.id, i.code, i.title, i.description, i.cate_id, coalesce(c.name,''),
	i.dept_id, coalesce(d.name,''), i.reporter_id, coalesce(ru.name,''), i.assignee_id,
	coalesce(au.name,''), i.severity, i.status, i.deadline, coalesce(i.result,''),
	coalesce(i.verify_comment,''), i.overdue, i.created_at, i.updated_at, i.closed_at,
	coalesce(i.post,'')`

const sqlIssueFrom = ` from issue i
	left join issue_cate c on c.id=i.cate_id
	left join org_dept d on d.id=i.dept_id
	left join org_user ru on ru.id=i.reporter_id
	left join org_user au on au.id=i.assignee_id `

func scanIssue(r rowScanner) (*model.Issue, error) {
	var i model.Issue
	err := r.Scan(&i.ID, &i.Code, &i.Title, &i.Description, &i.CateID, &i.CateName,
		&i.DeptID, &i.DeptName, &i.ReporterID, &i.Reporter, &i.AssigneeID, &i.Assignee,
		&i.Severity, &i.Status, &i.Deadline, &i.Result, &i.VerifyComment, &i.Overdue,
		&i.CreatedAt, &i.UpdatedAt, &i.ClosedAt, &i.Post)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

// CreateIssue 上报问题，编号形如 WT20251111-0007。
func (s *Store) CreateIssue(ctx context.Context, issue *model.Issue) (*model.Issue, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var id int64
	var createdAt time.Time
	err = tx.QueryRow(ctx, `insert into issue(code, title, description, cate_id, dept_id, post,
		reporter_id, assignee_id, severity, status, deadline)
		values ('PENDING-'||gen_random_uuid()::text, $1,$2,$3,$4,$5,$6,$7,$8,'reported',$9)
		returning id, created_at`,
		issue.Title, issue.Description, issue.CateID, issue.DeptID, issue.Post, issue.ReporterID,
		issue.AssigneeID, issue.Severity, issue.Deadline).Scan(&id, &createdAt)
	if err != nil {
		return nil, wrapErr("上报问题", err)
	}
	code := fmt.Sprintf("WT%s-%04d", createdAt.Format("20060102"), id)
	if _, err := tx.Exec(ctx, `update issue set code=$2 where id=$1`, id, code); err != nil {
		return nil, wrapErr("生成问题编号", err)
	}
	if _, err := tx.Exec(ctx, `insert into issue_flow(issue_id, action, from_status, to_status, actor_id, comment)
		values ($1,'report',null,'reported',$2,$3)`, id, issue.ReporterID, issue.Description); err != nil {
		return nil, wrapErr("写入流转记录", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetIssue(ctx, id)
}

// GetIssue 取问题详情。
func (s *Store) GetIssue(ctx context.Context, id int64) (*model.Issue, error) {
	i, err := scanIssue(s.pool.QueryRow(ctx, `select `+sqlIssueCols+sqlIssueFrom+` where i.id=$1`, id))
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return i, wrapErr("查询问题", err)
}

// ReassignIssue 更换整改负责人（仅限已受理状态），写流转和消息。
func (s *Store) ReassignIssue(ctx context.Context, issueID, actorID, assigneeID int64, comment string) (*model.Issue, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var status string
	var oldAssignee *int64
	var code, title string
	if err := tx.QueryRow(ctx, `select status, assignee_id, code, title from issue where id=$1 for update`, issueID).
		Scan(&status, &oldAssignee, &code, &title); err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, wrapErr("锁定问题", err)
	}
	if status != model.IssueAccepted {
		return nil, ErrConflict
	}
	tag, err := tx.Exec(ctx, `update issue set assignee_id=$2, updated_at=now() where id=$1 and status=$3`,
		issueID, assigneeID, model.IssueAccepted)
	if err != nil {
		return nil, wrapErr("更换负责人", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrConflict
	}
	if _, err := tx.Exec(ctx, `insert into issue_flow(issue_id, action, from_status, to_status, actor_id, comment)
		values ($1,'reassign',$2,$2,$3,$4)`, issueID, model.IssueAccepted, actorID, comment); err != nil {
		return nil, wrapErr("写入流转记录", err)
	}
	label := code + " " + title
	if _, err := tx.Exec(ctx, `insert into message(user_id, kind, title, body, ref_type, ref_id)
		values ($1,'issue','你被指定为整改负责人',$2,'issue',$3)`, assigneeID, label, issueID); err != nil {
		return nil, wrapErr("写入消息", err)
	}
	if oldAssignee != nil && *oldAssignee != assigneeID {
		if _, err := tx.Exec(ctx, `insert into message(user_id, kind, title, body, ref_type, ref_id)
			values ($1,'issue','整改任务已转交',$2,'issue',$3)`, *oldAssignee, label, issueID); err != nil {
			return nil, wrapErr("写入消息", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetIssue(ctx, issueID)
}

// ListIssuesByUser 列出“我上报的 + 指派给我的”问题。
func (s *Store) ListIssuesByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Issue, int, error) {
	rows, err := s.pool.Query(ctx, `select `+sqlIssueCols+`, count(*) over() as total`+sqlIssueFrom+`
		where (i.reporter_id=$1 or i.assignee_id=$1)
		order by i.id desc limit $2 offset $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询我的问题", err)
	}
	defer rows.Close()
	out := []model.Issue{}
	total := 0
	for rows.Next() {
		var i model.Issue
		if err := rows.Scan(&i.ID, &i.Code, &i.Title, &i.Description, &i.CateID, &i.CateName,
			&i.DeptID, &i.DeptName, &i.ReporterID, &i.Reporter, &i.AssigneeID, &i.Assignee,
			&i.Severity, &i.Status, &i.Deadline, &i.Result, &i.VerifyComment, &i.Overdue,
			&i.CreatedAt, &i.UpdatedAt, &i.ClosedAt, &i.Post, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, i)
	}
	return out, total, rows.Err()
}

// ListIssues 查询问题列表。
func (s *Store) ListIssues(ctx context.Context, status string, deptID, reporterID *int64, keyword string, limit, offset int) ([]model.Issue, int, error) {
	rows, err := s.pool.Query(ctx, `select `+sqlIssueCols+`, count(*) over() as total`+sqlIssueFrom+`
		where ($1='' or i.status=$1)
		  and ($2::bigint is null or i.dept_id=$2)
		  and ($3::bigint is null or i.reporter_id=$3)
		  and ($4='' or i.title ilike '%'||$4||'%' or i.code ilike '%'||$4||'%')
		order by i.id desc limit $5 offset $6`, status, deptID, reporterID, keyword, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询问题列表", err)
	}
	defer rows.Close()
	var out []model.Issue
	total := 0
	for rows.Next() {
		var i model.Issue
		if err := rows.Scan(&i.ID, &i.Code, &i.Title, &i.Description, &i.CateID, &i.CateName,
			&i.DeptID, &i.DeptName, &i.ReporterID, &i.Reporter, &i.AssigneeID, &i.Assignee,
			&i.Severity, &i.Status, &i.Deadline, &i.Result, &i.VerifyComment, &i.Overdue,
			&i.CreatedAt, &i.UpdatedAt, &i.ClosedAt, &i.Post, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, i)
	}
	return out, total, rows.Err()
}

// TransitionRequest 描述一次状态流转。
type TransitionRequest struct {
	IssueID       int64
	Action        string
	From          []string
	To            string
	ActorID       int64
	Comment       string
	Result        string
	VerifyComment string
	AssigneeID    *int64
	Deadline      *time.Time
	Close         bool
}

// TransitionIssue 在事务内做状态迁移，状态不匹配返回 ErrConflict。
// 流转记录只插入、不修改。
func (s *Store) TransitionIssue(ctx context.Context, req TransitionRequest) (*model.Issue, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var current string
	var reporter, assignee *int64
	var title, code string
	if err := tx.QueryRow(ctx, `select status, reporter_id, assignee_id, title, code from issue where id=$1 for update`,
		req.IssueID).Scan(&current, &reporter, &assignee, &title, &code); err != nil {
		if isNoRows(err) {
			return nil, ErrNotFound
		}
		return nil, wrapErr("锁定问题", err)
	}
	allowed := false
	for _, f := range req.From {
		if f == current {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, ErrConflict
	}

	tag, err := tx.Exec(ctx, `update issue set status=$2, updated_at=now(),
		result = coalesce(nullif($3,''), result),
		verify_comment = coalesce(nullif($4,''), verify_comment),
		assignee_id = coalesce($5, assignee_id),
		deadline = coalesce($8, deadline),
		overdue = case when $6 then false else overdue end,
		closed_at = case when $6 then now() else closed_at end
		where id=$1 and status=$7`,
		req.IssueID, req.To, req.Result, req.VerifyComment, req.AssigneeID, req.Close, current, req.Deadline)
	if err != nil {
		return nil, wrapErr("更新问题状态", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrConflict
	}
	if _, err := tx.Exec(ctx, `insert into issue_flow(issue_id, action, from_status, to_status, actor_id, comment)
		values ($1,$2,$3,$4,$5,$6)`, req.IssueID, req.Action, current, req.To, req.ActorID, req.Comment); err != nil {
		return nil, wrapErr("写入流转记录", err)
	}

	// 站内消息：受理/整改/验收/闭环/驳回通知相关人。
	type msg struct {
		to    *int64
		title string
		body  string
	}
	msgs := []msg{}
	label := code + " " + title
	comment := req.Comment
	if len(comment) > 60 {
		comment = comment[:60] + "…"
	}
	switch req.To {
	case model.IssueAccepted:
		assigneeToNotify := assignee
		if req.AssigneeID != nil {
			assigneeToNotify = req.AssigneeID
		}
		if current == model.IssueRectified {
			msgs = append(msgs, msg{assigneeToNotify, "验收不合格，请重新整改", label + "；" + comment})
		} else {
			msgs = append(msgs, msg{assigneeToNotify, "你有一条隐患整改任务", label})
			msgs = append(msgs, msg{reporter, "你的上报已受理", label})
		}
	case model.IssueRectified:
		msgs = append(msgs, msg{reporter, "隐患已整改，等待验收", label})
	case model.IssueVerified:
		msgs = append(msgs, msg{reporter, "隐患验收通过", label})
	case model.IssueClosed:
		msgs = append(msgs, msg{reporter, "隐患已闭环", label})
	case model.IssueRejected:
		msgs = append(msgs, msg{reporter, "上报被驳回", label + "；原因：" + comment})
	}
	for _, m := range msgs {
		if m.to == nil || *m.to == req.ActorID {
			continue
		}
		if _, err := tx.Exec(ctx, `insert into message(user_id, kind, title, body, ref_type, ref_id)
			values ($1,'issue',$2,$3,'issue',$4)`, *m.to, m.title, m.body, req.IssueID); err != nil {
			return nil, wrapErr("写入消息", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetIssue(ctx, req.IssueID)
}

// ListIssueFlows 查询流转记录（只读）。
func (s *Store) ListIssueFlows(ctx context.Context, issueID int64) ([]model.IssueFlow, error) {
	rows, err := s.pool.Query(ctx, `select f.id, f.issue_id, f.action, coalesce(f.from_status,''),
		f.to_status, f.actor_id, coalesce(u.name,''), coalesce(f.comment,''), f.created_at
		from issue_flow f left join org_user u on u.id=f.actor_id
		where f.issue_id=$1 order by f.id`, issueID)
	if err != nil {
		return nil, wrapErr("查询流转记录", err)
	}
	defer rows.Close()
	var out []model.IssueFlow
	for rows.Next() {
		var f model.IssueFlow
		if err := rows.Scan(&f.ID, &f.IssueID, &f.Action, &f.FromStatus, &f.ToStatus,
			&f.ActorID, &f.Actor, &f.Comment, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// LastFlowActor 返回某问题最近一次指定动作的操作人；没有记录返回 nil。
// actions 可传多个（例如正常与管理员强制两种 action）。
func (s *Store) LastFlowActor(ctx context.Context, issueID int64, actions ...string) (*int64, error) {
	var actor *int64
	err := s.pool.QueryRow(ctx, `select actor_id from issue_flow
		where issue_id=$1 and action = any($2) order by id desc limit 1`, issueID, actions).Scan(&actor)
	if isNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, wrapErr("查询流转记录", err)
	}
	return actor, nil
}

// AddIssueAttachment 记录附件。
func (s *Store) AddIssueAttachment(ctx context.Context, a *model.IssueAttachment) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `insert into issue_attachment(issue_id, file_name, stored_path,
		file_size, sha256, uploader_id, kind) values ($1,$2,$3,$4,nullif($5,''),$6,coalesce(nullif($7,''),'report')) returning id`,
		a.IssueID, a.FileName, a.FileName, a.FileSize, a.SHA256, a.UploaderID, a.Kind).Scan(&id)
	return id, wrapErr("写入附件", err)
}

// ListIssueAttachments 查询附件。
func (s *Store) ListIssueAttachments(ctx context.Context, issueID int64) ([]model.IssueAttachment, error) {
	rows, err := s.pool.Query(ctx, `select id, issue_id, file_name, file_size, coalesce(sha256,''),
		uploader_id, coalesce(kind,'report'), created_at from issue_attachment where issue_id=$1 order by id`, issueID)
	if err != nil {
		return nil, wrapErr("查询附件", err)
	}
	defer rows.Close()
	var out []model.IssueAttachment
	for rows.Next() {
		var a model.IssueAttachment
		if err := rows.Scan(&a.ID, &a.IssueID, &a.FileName, &a.FileSize, &a.SHA256, &a.UploaderID, &a.Kind, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// LastFlowComment 取最近一次指定动作的备注（例如驳回原因）。
func (s *Store) LastFlowComment(ctx context.Context, issueID int64, actions ...string) (string, error) {
	var comment string
	err := s.pool.QueryRow(ctx, `select coalesce(comment,'') from issue_flow
		where issue_id=$1 and action = any($2) order by id desc limit 1`, issueID, actions).Scan(&comment)
	if isNoRows(err) {
		return "", nil
	}
	if err != nil {
		return "", wrapErr("查询流转备注", err)
	}
	return comment, nil
}

// ResubmitIssue 被驳回后由上报人修改并重新提交，状态回到 reported。
func (s *Store) ResubmitIssue(ctx context.Context, issueID, userID int64, title, desc string) (*model.Issue, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `update issue set status='reported', title=$3, description=$4,
		updated_at=now(), overdue=false
		where id=$1 and reporter_id=$2 and status='rejected'`, issueID, userID, title, desc)
	if err != nil {
		return nil, wrapErr("重新提交问题", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrConflict
	}
	if _, err := tx.Exec(ctx, `insert into issue_flow(issue_id, action, from_status, to_status, actor_id, comment)
		values ($1,'resubmit','rejected','reported',$2,$3)`, issueID, userID, "修改后重新上报"); err != nil {
		return nil, wrapErr("写入流转记录", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetIssue(ctx, issueID)
}

// SweepOverdueIssues 标记超时问题。
func (s *Store) SweepOverdueIssues(ctx context.Context) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `update issue set overdue=true, updated_at=now()
		where status in ('reported','accepted','rectified') and deadline is not null
		  and deadline < now() and not overdue returning id`)
	if err != nil {
		return nil, wrapErr("扫描超时问题", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		// 超时提醒：责任人 + 本队队长 + 安全科
		if _, err := s.pool.Exec(ctx, `insert into message(user_id, kind, title, body, ref_type, ref_id)
			select u.id, 'issue', '隐患已超时', '问题 '||i.code||' '||i.title||' 超过整改时限，请尽快处理', 'issue', i.id
			from issue i join org_user u on u.id = coalesce(i.assignee_id, i.reporter_id)
			where i.id = any($1::bigint[])`, ids); err != nil {
			return ids, wrapErr("写入超时提醒", err)
		}
		if _, err := s.pool.Exec(ctx, `insert into message(user_id, kind, title, body, ref_type, ref_id)
			select u.id, 'issue', '隐患超时（管理提醒）', '问题 '||i.code||' '||i.title||' 已超时，请督促整改', 'issue', i.id
			from issue i join org_user u on u.role='dept_leader' and u.dept_id = i.dept_id
			where i.id = any($1::bigint[])`, ids); err != nil {
			return ids, wrapErr("写入超时管理提醒", err)
		}
		if _, err := s.pool.Exec(ctx, `insert into message(user_id, kind, title, body, ref_type, ref_id)
			select u.id, 'issue', '隐患超时（安全科）', '问题 '||i.code||' '||i.title||' 已超时', 'issue', i.id
			from issue i cross join org_user u
			where u.role='safety' and u.status='active' and i.id = any($1::bigint[])`, ids); err != nil {
			return ids, wrapErr("写入超时安全科提醒", err)
		}
	}
	return ids, nil
}

// IssueStats 返回各状态计数与超时数。
func (s *Store) IssueStats(ctx context.Context, deptID, reporterID *int64) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `select status, count(*) from issue
		where ($1::bigint is null or dept_id=$1)
		  and ($2::bigint is null or reporter_id=$2)
		group by status`, deptID, reporterID)
	if err != nil {
		return nil, wrapErr("统计问题", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[s] = n
	}
	var overdue int
	if err := s.pool.QueryRow(ctx, `select count(*) from issue
		where overdue and ($1::bigint is null or dept_id=$1)
		  and ($2::bigint is null or reporter_id=$2)`, deptID, reporterID).Scan(&overdue); err != nil {
		return nil, wrapErr("统计超时问题", err)
	}
	out["overdue"] = overdue
	return out, rows.Err()
}
