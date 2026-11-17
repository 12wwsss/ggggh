package store

import (
	"context"
	"encoding/json"

	"coalrag/internal/model"
)

// ListPapers 查询试卷。
func (s *Store) ListPapers(ctx context.Context, status string, deptID *int64, limit, offset int) ([]model.ExamPaper, int, error) {
	rows, err := s.pool.Query(ctx, `select id, title, dept_id, question_ids, total_score, pass_score,
		duration_min, start_at, end_at, status, created_by, created_at, max_attempts, count(*) over() as total
		from exam_paper
		where ($1='' or status=$1) and ($2::bigint is null or dept_id is null or dept_id=$2)
		order by id desc limit $3 offset $4`, status, deptID, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询试卷", err)
	}
	defer rows.Close()
	var out []model.ExamPaper
	total := 0
	for rows.Next() {
		var p model.ExamPaper
		if err := rows.Scan(&p.ID, &p.Title, &p.DeptID, &p.QuestionIDs, &p.TotalScore, &p.PassScore,
			&p.DurationMin, &p.StartAt, &p.EndAt, &p.Status, &p.CreatedBy, &p.CreatedAt, &p.MaxAttempts, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// GetPaper 取试卷。
func (s *Store) GetPaper(ctx context.Context, id int64) (*model.ExamPaper, error) {
	var p model.ExamPaper
	err := s.pool.QueryRow(ctx, `select id, title, dept_id, question_ids, total_score, pass_score,
		duration_min, start_at, end_at, status, created_by, created_at, max_attempts from exam_paper where id=$1`, id).
		Scan(&p.ID, &p.Title, &p.DeptID, &p.QuestionIDs, &p.TotalScore, &p.PassScore,
			&p.DurationMin, &p.StartAt, &p.EndAt, &p.Status, &p.CreatedBy, &p.CreatedAt, &p.MaxAttempts)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询试卷", err)
	}
	return &p, nil
}

// CreatePaper 新建试卷。
func (s *Store) CreatePaper(ctx context.Context, p *model.ExamPaper) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `insert into exam_paper
		(title, dept_id, question_ids, total_score, pass_score, duration_min, start_at, end_at, status, created_by, max_attempts)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) returning id`,
		p.Title, p.DeptID, arr64(p.QuestionIDs), p.TotalScore, p.PassScore, p.DurationMin,
		p.StartAt, p.EndAt, p.Status, p.CreatedBy, p.MaxAttempts).Scan(&id)
	return id, wrapErr("新建试卷", err)
}

// SetPaperStatus 变更试卷状态。
func (s *Store) SetPaperStatus(ctx context.Context, id int64, status string) error {
	tag, err := s.pool.Exec(ctx, `update exam_paper set status=$2 where id=$1`, id, status)
	if err != nil {
		return wrapErr("更新试卷状态", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// StartExam 开始一次考试。
// 已有进行中的记录直接返回（幂等）；允许次数用完返回 ErrConflict；
// 否则按已用次数生成下一条（attempt_no 递增）。
func (s *Store) StartExam(ctx context.Context, paperID, userID int64, maxAttempts int) (*model.ExamRecord, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var answers []byte
	var r model.ExamRecord
	err = tx.QueryRow(ctx, `select id, paper_id, user_id, answers, score, passed, status, attempt_no,
		started_at, submitted_at from exam_record
		where paper_id=$1 and user_id=$2 and status='in_progress'
		order by attempt_no desc limit 1 for update`, paperID, userID).
		Scan(&r.ID, &r.PaperID, &r.UserID, &answers, &r.Score, &r.Passed, &r.Status, &r.AttemptNo,
			&r.StartedAt, &r.SubmittedAt)
	if err == nil {
		_ = json.Unmarshal(answers, &r.Answers)
		if r.Answers == nil {
			r.Answers = map[string]string{}
		}
		return &r, tx.Commit(ctx)
	}
	if !isNoRows(err) {
		return nil, wrapErr("查询考试记录", err)
	}

	var used int
	if err := tx.QueryRow(ctx, `select count(*) from exam_record where paper_id=$1 and user_id=$2`,
		paperID, userID).Scan(&used); err != nil {
		return nil, wrapErr("统计考试次数", err)
	}
	if used >= maxAttempts {
		return nil, ErrConflict
	}

	err = tx.QueryRow(ctx, `insert into exam_record(paper_id, user_id, attempt_no)
		values($1,$2,$3)
		returning id, paper_id, user_id, answers, score, passed, status, attempt_no, started_at, submitted_at`,
		paperID, userID, used+1).
		Scan(&r.ID, &r.PaperID, &r.UserID, &answers, &r.Score, &r.Passed, &r.Status, &r.AttemptNo,
			&r.StartedAt, &r.SubmittedAt)
	if err != nil {
		return nil, wrapErr("开始考试", err)
	}
	_ = json.Unmarshal(answers, &r.Answers)
	if r.Answers == nil {
		r.Answers = map[string]string{}
	}
	return &r, tx.Commit(ctx)
}

// SetPaperMaxAttempts 设置试卷允许的考试次数（发布前调用）。
func (s *Store) SetPaperMaxAttempts(ctx context.Context, id int64, n int) error {
	if n <= 0 {
		n = 1
	}
	tag, err := s.pool.Exec(ctx, `update exam_paper set max_attempts=$2 where id=$1 and status='draft'`, id, n)
	if err != nil {
		return wrapErr("更新考试次数", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetLatestExamRecord 取某人某试卷最近一次记录（不限状态），用于“查看成绩”。
func (s *Store) GetLatestExamRecord(ctx context.Context, paperID, userID int64) (*model.ExamRecord, error) {
	var r model.ExamRecord
	var answers []byte
	err := s.pool.QueryRow(ctx, `select id, paper_id, user_id, answers, score, passed, status, attempt_no,
		started_at, submitted_at from exam_record
		where paper_id=$1 and user_id=$2 order by id desc limit 1`, paperID, userID).
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

// CountExamAttempts 统计某人在某试卷上已经产生的记录数（含进行中）。
func (s *Store) CountExamAttempts(ctx context.Context, paperID, userID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from exam_record where paper_id=$1 and user_id=$2`,
		paperID, userID).Scan(&n)
	return n, wrapErr("统计考试次数", err)
}

// SubmitExam 交卷；重复提交返回冲突。
func (s *Store) SubmitExam(ctx context.Context, recordID int64, answers map[string]string, score int, passed bool) error {
	b, _ := json.Marshal(answers)
	tag, err := s.pool.Exec(ctx, `update exam_record set answers=$2, score=$3, passed=$4,
		status='submitted', submitted_at=now() where id=$1 and status='in_progress'`, recordID, b, score, passed)
	if err != nil {
		return wrapErr("提交试卷", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

// GetExamRecordByPaper 取某人当前进行中的考试记录。
func (s *Store) GetExamRecordByPaper(ctx context.Context, paperID, userID int64) (*model.ExamRecord, error) {
	var r model.ExamRecord
	var answers []byte
	err := s.pool.QueryRow(ctx, `select id, paper_id, user_id, answers, score, passed, status, attempt_no,
		started_at, submitted_at from exam_record
		where paper_id=$1 and user_id=$2 and status='in_progress'
		order by attempt_no desc limit 1`, paperID, userID).
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

// ListExamRecords 按范围查询考试记录。
func (s *Store) ListExamRecords(ctx context.Context, paperID, deptID, userID *int64, limit, offset int) ([]model.ExamRecord, int, error) {
	rows, err := s.pool.Query(ctx, `select r.id, r.paper_id, p.title, r.user_id, u.name, coalesce(d.name,''),
		r.answers, r.score, r.passed, r.status, r.attempt_no, r.started_at, r.submitted_at, count(*) over() as total
		from exam_record r
		join exam_paper p on p.id=r.paper_id
		join org_user u on u.id=r.user_id
		left join org_dept d on d.id=u.dept_id
		where ($1::bigint is null or r.paper_id=$1)
		  and ($2::bigint is null or u.dept_id=$2)
		  and ($3::bigint is null or r.user_id=$3)
		order by r.id desc limit $4 offset $5`, paperID, deptID, userID, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询考试记录", err)
	}
	defer rows.Close()
	out := []model.ExamRecord{}
	total := 0
	for rows.Next() {
		var r model.ExamRecord
		var answers []byte
		if err := rows.Scan(&r.ID, &r.PaperID, &r.PaperTitle, &r.UserID, &r.UserName, &r.DeptName,
			&answers, &r.Score, &r.Passed, &r.Status, &r.AttemptNo, &r.StartedAt, &r.SubmittedAt, &total); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(answers, &r.Answers)
		if r.Answers == nil {
			r.Answers = map[string]string{}
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}
