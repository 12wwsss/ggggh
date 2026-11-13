package store

import (
	"context"
	"time"
)

// SweepExamTimeouts 把超过考试时长仍未交卷的记录置为超时：
// 状态改 timeout、按 0 分计，统计里视为已交卷但未通过。
// 返回被处理的记录 ID，便于日志与审计。
func (s *Store) SweepExamTimeouts(ctx context.Context) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `update exam_record r
		set status = 'timeout',
		    submitted_at = now(),
		    score = 0,
		    passed = false
		from exam_paper p
		where p.id = r.paper_id
		  and r.status = 'in_progress'
		  and r.started_at + make_interval(mins => p.duration_min) < now()
		returning r.id, r.paper_id, r.user_id`)
	if err != nil {
		return nil, wrapErr("扫描考试超时", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id, paperID, userID int64
		if err := rows.Scan(&id, &paperID, &userID); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ExamRecordStartedAt 返回记录的开始时间，测试与排查用。
func (s *Store) ExamRecordStartedAt(ctx context.Context, id int64) (time.Time, error) {
	var t time.Time
	err := s.pool.QueryRow(ctx, `select started_at from exam_record where id=$1`, id).Scan(&t)
	return t, wrapErr("查询考试开始时间", err)
}
