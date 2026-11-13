package store

import (
	"context"
	"time"
)

// PurgeFinishedTasks 删除超过保留期的已结束任务，避免任务表无限增长。
// 只删 succeeded / failed，排队中与执行中的任务不动。
func (s *Store) PurgeFinishedTasks(ctx context.Context, olderThan time.Duration) (int64, error) {
	if olderThan <= 0 {
		olderThan = 7 * 24 * time.Hour
	}
	tag, err := s.pool.Exec(ctx, `delete from task
		where status in ('succeeded','failed')
		  and created_at < now() - $1::interval`, durationExpr(olderThan))
	if err != nil {
		return 0, wrapErr("清理历史任务", err)
	}
	return tag.RowsAffected(), nil
}

// TaskStats 返回任务表总量与各状态计数，用于运维排查。
func (s *Store) TaskStats(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `select status, count(*) from task group by status`)
	if err != nil {
		return nil, wrapErr("统计任务", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}
