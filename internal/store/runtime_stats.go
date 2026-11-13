package store

import "context"

// RuntimeStats 是抓取指标时用的即时计数。
type RuntimeStats struct {
	Docs              int `json:"docs"`
	Chunks            int `json:"chunks"`
	Users             int `json:"users"`
	TasksPending      int `json:"tasks_pending"`
	TasksFailed       int `json:"tasks_failed"`
	TasksFailedRecent int `json:"tasks_failed_recent"`
	IssuesOpen        int `json:"issues_open"`
	QAAnswered        int `json:"qa_answered"`
	QAFallback        int `json:"qa_fallback"`
	QuestionsToday    int `json:"questions_today"`
}

// RuntimeStats 汇总运行指标（一次查询拿全，避免多次往返）。
func (s *Store) RuntimeStats(ctx context.Context) (*RuntimeStats, error) {
	var st RuntimeStats
	err := s.pool.QueryRow(ctx, `select
		(select count(*) from doc where status='published'),
		(select count(*) from doc_chunk where published),
		(select count(*) from org_user where status='active'),
		(select count(*) from task where status='pending'),
		(select count(*) from task where status='failed'),
		(select count(*) from task where status='failed' and created_at > now() - interval '1 hour'),
		(select count(*) from issue where status not in ('verified','closed','rejected')),
		(select count(*) from qa_log where answered),
		(select count(*) from qa_log where not answered),
		(select count(*) from qa_log where created_at >= date_trunc('day', now()))`).
		Scan(&st.Docs, &st.Chunks, &st.Users, &st.TasksPending, &st.TasksFailed,
			&st.TasksFailedRecent, &st.IssuesOpen, &st.QAAnswered, &st.QAFallback, &st.QuestionsToday)
	if err != nil {
		return nil, wrapErr("统计运行指标", err)
	}
	return &st, nil
}
