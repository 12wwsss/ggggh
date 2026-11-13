package store

import (
	"context"
)

// Dashboard 是三指标看板的数据。
type Dashboard struct {
	LearnCompletion  float64 `json:"learn_completion"`
	LearnCompleted   int     `json:"learn_completed"`
	LearnExpected    int     `json:"learn_expected"`
	ExamPassRate     float64 `json:"exam_pass_rate"`
	ExamPassed       int     `json:"exam_passed"`
	ExamSubmitted    int     `json:"exam_submitted"`
	BroadcastAckRate float64 `json:"broadcast_ack_rate"`
	BroadcastAck     int     `json:"broadcast_ack"`
	BroadcastTotal   int     `json:"broadcast_total"`
	Docs             int     `json:"docs"`
	Chunks           int     `json:"chunks"`
	QATotal          int     `json:"qa_total"`
	QAToday          int     `json:"qa_today"`
	QAFallbackRate   float64 `json:"qa_fallback_rate"`
	IssuesOpen       int     `json:"issues_open"`
	IssuesOverdue    int     `json:"issues_overdue"`
}

// DashboardData 汇总三指标与基础计数。deptID 为空表示全矿。
func (s *Store) DashboardData(ctx context.Context, deptID *int64) (*Dashboard, error) {
	d := &Dashboard{}

	// 学习完成率 = 已完成记录数 / (在册人数 × 已发布条目数)
	if err := s.pool.QueryRow(ctx, `select count(*) from org_user
		where status='active' and ($1::bigint is null or dept_id=$1)`, deptID).Scan(&d.LearnExpected); err != nil {
		return nil, wrapErr("统计在册人数", err)
	}
	var items int
	if err := s.pool.QueryRow(ctx, `select count(*) from learn_item where status='published'`).Scan(&items); err != nil {
		return nil, wrapErr("统计学习条目", err)
	}
	d.LearnExpected *= items
	if err := s.pool.QueryRow(ctx, `select count(*) from learn_record r
		join org_user u on u.id=r.user_id join learn_item i on i.id=r.item_id
		where r.status='completed' and i.status='published'
		  and ($1::bigint is null or u.dept_id=$1)`, deptID).Scan(&d.LearnCompleted); err != nil {
		return nil, wrapErr("统计学习完成", err)
	}
	d.LearnCompletion = ratio(d.LearnCompleted, d.LearnExpected)

	// 考试通过率 = 通过人数 / 已交卷人数
	if err := s.pool.QueryRow(ctx, `select count(*) from exam_record r
		join org_user u on u.id=r.user_id
		where r.status in ('submitted','timeout') and ($1::bigint is null or u.dept_id=$1)`,
		deptID).Scan(&d.ExamSubmitted); err != nil {
		return nil, wrapErr("统计交卷数", err)
	}
	if err := s.pool.QueryRow(ctx, `select count(*) from exam_record r
		join org_user u on u.id=r.user_id
		where r.passed and r.status in ('submitted','timeout') and ($1::bigint is null or u.dept_id=$1)`,
		deptID).Scan(&d.ExamPassed); err != nil {
		return nil, wrapErr("统计通过数", err)
	}
	d.ExamPassRate = ratio(d.ExamPassed, d.ExamSubmitted)

	// 宣贯签收率
	if err := s.pool.QueryRow(ctx, `select count(*) from broadcast_record r
		join broadcast_task t on t.id=r.task_id
		join org_user u on u.id=r.user_id
		where t.status='published' and ($1::bigint is null or u.dept_id=$1)`, deptID).Scan(&d.BroadcastTotal); err != nil {
		return nil, wrapErr("统计宣贯总数", err)
	}
	if err := s.pool.QueryRow(ctx, `select count(*) from broadcast_record r
		join broadcast_task t on t.id=r.task_id
		join org_user u on u.id=r.user_id
		where t.status='published' and r.status='ack' and ($1::bigint is null or u.dept_id=$1)`,
		deptID).Scan(&d.BroadcastAck); err != nil {
		return nil, wrapErr("统计签收数", err)
	}
	d.BroadcastAckRate = ratio(d.BroadcastAck, d.BroadcastTotal)

	if err := s.pool.QueryRow(ctx, `select count(*) from doc where status='published'`).Scan(&d.Docs); err != nil {
		return nil, wrapErr("统计文档", err)
	}
	if err := s.pool.QueryRow(ctx, `select count(*) from doc_chunk where published`).Scan(&d.Chunks); err != nil {
		return nil, wrapErr("统计切片", err)
	}

	// 问答量与兜底率
	var qaAll, qaAnswered int
	if err := s.pool.QueryRow(ctx, `select count(*), count(*) filter (where answered) from qa_log q
		join org_user u on u.id=q.user_id
		where ($1::bigint is null or u.dept_id=$1)`, deptID).Scan(&qaAll, &qaAnswered); err != nil {
		return nil, wrapErr("统计问答", err)
	}
	d.QATotal = qaAll
	d.QAFallbackRate = ratio(qaAll-qaAnswered, qaAll)
	if err := s.pool.QueryRow(ctx, `select count(*) from qa_log q
		join org_user u on u.id=q.user_id
		where q.created_at >= date_trunc('day', now()) and ($1::bigint is null or u.dept_id=$1)`,
		deptID).Scan(&d.QAToday); err != nil {
		return nil, wrapErr("统计今日问答", err)
	}

	// 问题闭环
	if err := s.pool.QueryRow(ctx, `select count(*) from issue
		where status not in ('verified','closed','rejected') and ($1::bigint is null or dept_id=$1)`,
		deptID).Scan(&d.IssuesOpen); err != nil {
		return nil, wrapErr("统计未闭环问题", err)
	}
	if err := s.pool.QueryRow(ctx, `select count(*) from issue
		where overdue and status not in ('verified','closed') and ($1::bigint is null or dept_id=$1)`,
		deptID).Scan(&d.IssuesOverdue); err != nil {
		return nil, wrapErr("统计超时问题", err)
	}
	return d, nil
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
