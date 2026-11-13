package store

import "context"

// PendingBroadcastCount 统计已过截止时间且仍未签收的人数。
func (s *Store) PendingBroadcastCount(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from broadcast_record r
		join broadcast_task t on t.id=r.task_id
		where t.status='published' and t.require_ack and r.status<>'ack'
		  and t.deadline is not null and t.deadline < now()`).Scan(&n)
	return n, wrapErr("统计逾期未签收", err)
}
