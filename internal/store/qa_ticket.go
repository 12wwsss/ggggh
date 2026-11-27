package store

import (
	"context"
	"time"
)

// QaTicket 是一条「转人工」工单。
type QaTicket struct {
	ID         int64      `json:"id"`
	LogID      *int64     `json:"log_id,omitempty"`
	UserID     *int64     `json:"user_id,omitempty"`
	DeptID     *int64     `json:"dept_id,omitempty"`
	UserName   string     `json:"user_name"`
	DeptName   string     `json:"dept_name"`
	Question   string     `json:"question"`
	Answer     string     `json:"answer"`
	Status     string     `json:"status"`
	AssigneeID *int64     `json:"assignee_id,omitempty"`
	Assignee   string     `json:"assignee"`
	Reply      string     `json:"reply"`
	CreatedAt  time.Time  `json:"created_at"`
	RepliedAt  *time.Time `json:"replied_at,omitempty"`
}

const qaTicketCols = `select t.id, t.log_id, t.user_id, coalesce(u.name,''), coalesce(d.name,''),
	t.question, coalesce(t.answer,''), t.status, t.assignee_id, coalesce(a.name,''),
	coalesce(t.reply,''), t.created_at, t.replied_at
	from qa_ticket t
	left join org_user u on u.id=t.user_id
	left join org_dept d on d.id=t.dept_id
	left join org_user a on a.id=t.assignee_id`

func scanQaTicket(row interface{ Scan(...any) error }, t *QaTicket) error {
	return row.Scan(&t.ID, &t.LogID, &t.UserID, &t.UserName, &t.DeptName, &t.Question,
		&t.Answer, &t.Status, &t.AssigneeID, &t.Assignee, &t.Reply, &t.CreatedAt, &t.RepliedAt)
}

// CreateQaTicket 新建转人工工单。
func (s *Store) CreateQaTicket(ctx context.Context, t *QaTicket) (int64, error) {
	if t.Status == "" {
		t.Status = "pending"
	}
	var id int64
	err := s.pool.QueryRow(ctx, `insert into qa_ticket(log_id, user_id, dept_id, question, answer, status)
		values ($1,$2,$3,$4,$5,$6) returning id`,
		t.LogID, t.UserID, t.DeptID, t.Question, t.Answer, t.Status).Scan(&id)
	return id, wrapErr("写入转人工工单", err)
}

// GetQaTicket 按 id 取工单。
func (s *Store) GetQaTicket(ctx context.Context, id int64) (*QaTicket, error) {
	var t QaTicket
	err := scanQaTicket(s.pool.QueryRow(ctx, qaTicketCols+` where t.id=$1`, id), &t)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询转人工工单", err)
	}
	return &t, nil
}

// ListQaTickets 工单列表；userID 非空时只看某个员工自己的。
func (s *Store) ListQaTickets(ctx context.Context, status, keyword string, userID *int64, limit, offset int) ([]QaTicket, int, error) {
	where := ` where ($1='' or t.status=$1)
		and ($2='' or t.question ilike '%'||$2||'%' or u.name ilike '%'||$2||'%')
		and ($3::bigint is null or t.user_id=$3)`
	rows, err := s.pool.Query(ctx, qaTicketCols+where+` order by t.id desc limit $4 offset $5`,
		status, keyword, userID, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询转人工工单", err)
	}
	defer rows.Close()
	out := []QaTicket{}
	for rows.Next() {
		var t QaTicket
		if err := scanQaTicket(rows, &t); err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	var total int
	err = s.pool.QueryRow(ctx, `select count(*) from qa_ticket t
		left join org_user u on u.id=t.user_id`+where, status, keyword, userID).Scan(&total)
	return out, total, wrapErr("统计转人工工单", err)
}

// ReplyQaTicket 安全科回复工单。
func (s *Store) ReplyQaTicket(ctx context.Context, id, assigneeID int64, reply string) error {
	ct, err := s.pool.Exec(ctx, `update qa_ticket set status='replied', assignee_id=$2,
		reply=$3, replied_at=now(), updated_at=now() where id=$1`, id, assigneeID, reply)
	if err != nil {
		return wrapErr("回复转人工工单", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CloseQaTicket 关闭工单。
func (s *Store) CloseQaTicket(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx, `update qa_ticket set status='closed', updated_at=now() where id=$1`, id)
	return wrapErr("关闭转人工工单", err)
}

// CountPendingQaTickets 待回复数量（后台角标用）。
func (s *Store) CountPendingQaTickets(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from qa_ticket where status='pending'`).Scan(&n)
	return n, wrapErr("统计待回复工单", err)
}
