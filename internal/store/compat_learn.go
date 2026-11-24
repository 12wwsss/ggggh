package store

import (
	"context"
	"encoding/json"
	"strings"

	"coalrag/internal/model"
)

// CompatQuestion 是给小程序题库用的题目视图（选项去掉 A. 前缀）。
type CompatQuestion struct {
	ID       int64    `json:"id"`
	Stem     string   `json:"stem"`
	Options  []string `json:"options"`
	Answer   string   `json:"answer"`
	Analysis string   `json:"analysis"`
	Job      string   `json:"job"`
	Category string   `json:"category"`
}

// ListCompatQuestions 按岗位/分类取题；all=false 时随机取 count 道。
func (s *Store) ListCompatQuestions(ctx context.Context, job, cat string, all bool, count int) ([]CompatQuestion, error) {
	if count <= 0 {
		count = 5
	}
	q := `select q.id, q.stem, q.options, q.answer, coalesce(q.analysis,''),
		coalesce(nullif(q.job,''), li.post, ''), coalesce(nullif(q.category,''), li.category, '')
		from quiz_question q left join learn_item li on li.id=q.item_id
		where ($1='' or q.job=$1 or li.post=$1 or coalesce(q.job,li.post,'')='' or coalesce(q.job,li.post,'')='通用')
		  and ($2='' or q.category=$2 or li.category=$2)
		order by q.id`
	args := []any{job, cat}
	if !all {
		q = `select * from (` + q + `) t order by random() limit $3`
		args = append(args, count)
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, wrapErr("查询题库", err)
	}
	defer rows.Close()
	out := []CompatQuestion{}
	for rows.Next() {
		var c CompatQuestion
		var opts []byte
		if err := rows.Scan(&c.ID, &c.Stem, &opts, &c.Answer, &c.Analysis, &c.Job, &c.Category); err != nil {
			return nil, err
		}
		var list []string
		_ = json.Unmarshal(opts, &list)
		c.Options = StripOptionPrefix(list)
		out = append(out, c)
	}
	return out, rows.Err()
}

// StripOptionPrefix 去掉 "A. " 这类前缀，页面自己会显示 A/B/C/D。
func StripOptionPrefix(opts []string) []string {
	out := make([]string, 0, len(opts))
	for _, o := range opts {
		o = strings.TrimSpace(o)
		if len(o) >= 3 && o[0] >= 'A' && o[0] <= 'F' && o[1] == '.' {
			o = strings.TrimSpace(o[2:])
		}
		out = append(out, o)
	}
	return out
}

// CompatQuestionGroups 按岗位统计题量。
func (s *Store) CompatQuestionGroups(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `select coalesce(nullif(q.job,''), nullif(li.post,''), '通用') as job, count(*)
		from quiz_question q left join learn_item li on li.id=q.item_id
		group by 1 order by 2 desc`)
	if err != nil {
		return nil, wrapErr("统计题库分组", err)
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var job string
		var n int
		if err := rows.Scan(&job, &n); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"name": job, "count": n})
	}
	return out, rows.Err()
}

// AddQuizAttempt 记录一次练题成绩。
func (s *Store) AddQuizAttempt(ctx context.Context, userID int64, job string, score, total int, wrong []byte) error {
	_, err := s.pool.Exec(ctx, `insert into quiz_attempt(user_id, job, score, total, wrong)
		values ($1,$2,$3,$4,$5)`, userID, job, score, total, wrong)
	return wrapErr("记录答题", err)
}

// QuizAttemptStats 返回本人答题次数与最好成绩。
func (s *Store) QuizAttemptStats(ctx context.Context, userID int64) (attempts, best int, err error) {
	err = s.pool.QueryRow(ctx, `select count(*), coalesce(max(score),0) from quiz_attempt where user_id=$1`,
		userID).Scan(&attempts, &best)
	if err != nil {
		return 0, 0, wrapErr("统计答题", err)
	}
	return attempts, best, nil
}

// QuizAttemptStatsFull 返回次数、最好成绩、平均分（百分制）和及格率。
func (s *Store) QuizAttemptStatsFull(ctx context.Context, userID int64) (attempts, best, bestPct int, avgPct, passRate float64, err error) {
	var avg, pass float64
	err = s.pool.QueryRow(ctx, `select count(*), coalesce(max(score),0),
		coalesce(max(case when total>0 then round(score*100.0/total) end),0),
		coalesce(avg(case when total>0 then score*100.0/total end),0),
		coalesce(avg(case when total>0 and score*100.0/total>=60 then 100 else 0 end),0)
		from quiz_attempt where user_id=$1`, userID).Scan(&attempts, &best, &bestPct, &avg, &pass)
	if err != nil {
		return 0, 0, 0, 0, 0, wrapErr("统计答题", err)
	}
	return attempts, best, bestPct, avg, pass, nil
}

// MarkLearnRead 记录学习完成（按条目+人 upsert）。
func (s *Store) MarkLearnRead(ctx context.Context, userID, itemID int64, minutes int) error {
	if minutes <= 0 {
		minutes = 1
	}
	_, err := s.pool.Exec(ctx, `insert into learn_record(item_id, user_id, status, progress, duration_sec, started_at, completed_at, updated_at)
		values ($1,$2,'completed',100,$3,now(),now(),now())
		on conflict (item_id, user_id) do update set
			status='completed', progress=100,
			duration_sec = learn_record.duration_sec + $3,
			completed_at = coalesce(learn_record.completed_at, now()),
			updated_at = now()`, itemID, userID, minutes*60)
	return wrapErr("记录学习", err)
}

// LearnCompletedCount 当前用户已完成的条目数。
func (s *Store) LearnCompletedCount(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from learn_record where user_id=$1 and status='completed'`, userID).Scan(&n)
	return n, wrapErr("统计学习", err)
}

// LearnStats 学习完成数与时长达标情况。
func (s *Store) LearnStats(ctx context.Context, userID int64) (completed int, seconds int, err error) {
	err = s.pool.QueryRow(ctx, `select count(*) filter (where status='completed'),
		coalesce(sum(duration_sec),0) from learn_record where user_id=$1`, userID).
		Scan(&completed, &seconds)
	if err != nil {
		return 0, 0, wrapErr("统计学习", err)
	}
	return completed, seconds, nil
}

// GetExamRecordByID 按记录 id 取考试记录（小程序 sessionId 用）。
func (s *Store) GetExamRecordByID(ctx context.Context, id int64) (*model.ExamRecord, error) {
	var r model.ExamRecord
	var answers []byte
	err := s.pool.QueryRow(ctx, `select id, paper_id, user_id, answers, score, passed, status, attempt_no,
		started_at, submitted_at from exam_record where id=$1`, id).
		Scan(&r.ID, &r.PaperID, &r.UserID, &answers, &r.Score, &r.Passed, &r.Status, &r.AttemptNo,
			&r.StartedAt, &r.SubmittedAt)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询考试记录", err)
	}
	_ = json.Unmarshal(answers, &r.Answers)
	if r.Answers == nil {
		r.Answers = map[string]string{}
	}
	return &r, nil
}

// ListMyExamRecords 某人的全部考试记录（用于考试列表状态）。
func (s *Store) ListMyExamRecords(ctx context.Context, userID int64) ([]model.ExamRecord, error) {
	rows, err := s.pool.Query(ctx, `select id, paper_id, user_id, answers, score, passed, status, attempt_no,
		started_at, submitted_at from exam_record where user_id=$1 order by id desc`, userID)
	if err != nil {
		return nil, wrapErr("查询考试记录", err)
	}
	defer rows.Close()
	out := []model.ExamRecord{}
	for rows.Next() {
		var r model.ExamRecord
		var answers []byte
		if err := rows.Scan(&r.ID, &r.PaperID, &r.UserID, &answers, &r.Score, &r.Passed, &r.Status,
			&r.AttemptNo, &r.StartedAt, &r.SubmittedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
