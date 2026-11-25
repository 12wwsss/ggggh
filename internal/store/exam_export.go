package store

import (
	"context"

	"coalrag/internal/model"
)

// ExamRecordsForExport 某张试卷的全部成绩（含姓名/部门）。
func (s *Store) ExamRecordsForExport(ctx context.Context, paperID int64) ([]model.ExamRecord, error) {
	rows, err := s.pool.Query(ctx, `select r.id, r.paper_id, coalesce(p.title,''), r.user_id,
		coalesce(u.name,''), coalesce(d.name,''), r.score, r.passed, r.status, r.attempt_no,
		r.started_at, r.submitted_at
		from exam_record r
		left join org_user u on u.id=r.user_id
		left join org_dept d on d.id=u.dept_id
		left join exam_paper p on p.id=r.paper_id
		where r.paper_id=$1 order by r.score desc, r.id`, paperID)
	if err != nil {
		return nil, wrapErr("查询考试成绩", err)
	}
	defer rows.Close()
	out := []model.ExamRecord{}
	for rows.Next() {
		var r model.ExamRecord
		if err := rows.Scan(&r.ID, &r.PaperID, &r.PaperTitle, &r.UserID, &r.UserName, &r.DeptName,
			&r.Score, &r.Passed, &r.Status, &r.AttemptNo, &r.StartedAt, &r.SubmittedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
