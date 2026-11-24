package store

import (
	"context"

	"coalrag/internal/model"
)

// LearningRow 学习档案一行。
type LearningRow struct {
	UserID    int64  `json:"user_id"`
	Name      string `json:"name"`
	Dept      string `json:"dept"`
	Completed int    `json:"completed"`
	Hours     int    `json:"hours"`
	LastAt    string `json:"last_at"`
}

// LearningReport 学习档案报表。
type LearningReport struct {
	Rows      []LearningRow `json:"rows"`
	Users     int           `json:"users"`
	Items     int           `json:"items"`
	Completed int           `json:"completed"`
}

// LearningReport 按人统计学习完成情况。
func (s *Store) LearningReport(ctx context.Context, deptID *int64) (*LearningReport, error) {
	rows, err := s.pool.Query(ctx, `select u.id, u.name, coalesce(d.name,''),
		count(r.id) filter (where r.status='completed'),
		coalesce(sum(r.duration_sec),0),
		coalesce(to_char(max(r.updated_at),'MM-DD HH24:MI'),'')
		from org_user u left join learn_record r on r.user_id=u.id
		left join org_dept d on d.id=u.dept_id
		where u.status='active' and ($1::bigint is null or u.dept_id=$1)
		group by u.id, u.name, d.name order by 4 desc, u.id`, deptID)
	if err != nil {
		return nil, wrapErr("统计学习档案", err)
	}
	defer rows.Close()
	rep := &LearningReport{Rows: []LearningRow{}}
	for rows.Next() {
		var r LearningRow
		var secs int
		if err := rows.Scan(&r.UserID, &r.Name, &r.Dept, &r.Completed, &secs, &r.LastAt); err != nil {
			return nil, err
		}
		r.Hours = secs / 3600
		rep.Completed += r.Completed
		rep.Rows = append(rep.Rows, r)
	}
	rep.Users = len(rep.Rows)
	_ = s.pool.QueryRow(ctx, `select count(*) from learn_item where status='published'`).Scan(&rep.Items)
	return rep, rows.Err()
}

// WrongRow 错题统计一行。
type WrongRow struct {
	QuestionID int64  `json:"question_id"`
	Stem       string `json:"stem"`
	Job        string `json:"job"`
	Category   string `json:"category"`
	Times      int    `json:"times"`
	Users      int    `json:"users"`
}

// WrongReport 错题统计。
type WrongReport struct {
	Rows    []WrongRow `json:"rows"`
	Records int        `json:"records"`
	Users   int        `json:"users"`
}

// WrongReport 按题统计答错次数。
func (s *Store) WrongReport(ctx context.Context, limit int) (*WrongReport, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := s.pool.Query(ctx, `select wb.question_id, q.stem,
		coalesce(nullif(q.job,''), nullif(li.post,''), '通用'),
		coalesce(nullif(q.category,''), li.category, ''),
		sum(wb.times), count(distinct wb.user_id)
		from wrong_book wb join quiz_question q on q.id=wb.question_id
		left join learn_item li on li.id=q.item_id
		group by 1,2,3,4 order by 5 desc limit $1`, limit)
	if err != nil {
		return nil, wrapErr("统计错题", err)
	}
	defer rows.Close()
	rep := &WrongReport{Rows: []WrongRow{}}
	for rows.Next() {
		var r WrongRow
		if err := rows.Scan(&r.QuestionID, &r.Stem, &r.Job, &r.Category, &r.Times, &r.Users); err != nil {
			return nil, err
		}
		rep.Rows = append(rep.Rows, r)
	}
	_ = s.pool.QueryRow(ctx, `select coalesce(sum(times),0), count(distinct user_id) from wrong_book`).
		Scan(&rep.Records, &rep.Users)
	return rep, rows.Err()
}

// DailyCount 每日上报/闭环数。
type DailyCount struct {
	Date     string `json:"date"`
	Reported int    `json:"reported"`
	Closed   int    `json:"closed"`
}

// DeptIssueRow 部门隐患统计。
type DeptIssueRow struct {
	Dept   string `json:"dept"`
	Total  int    `json:"total"`
	Closed int    `json:"closed"`
	Open   int    `json:"open"`
}

// IssueReport 隐患趋势。
type IssueReport struct {
	Daily      []DailyCount   `json:"daily"`
	Status     map[string]int `json:"status"`
	Overdue    int            `json:"overdue"`
	Total      int            `json:"total"`
	Closed     int            `json:"closed"`
	AvgCloseHr float64        `json:"avg_close_hours"`
	ByDept     []DeptIssueRow `json:"by_dept"`
}

// IssueReport 近 N 天隐患趋势与分布。
func (s *Store) IssueReport(ctx context.Context, days int) (*IssueReport, error) {
	if days <= 0 || days > 90 {
		days = 14
	}
	rep := &IssueReport{Daily: []DailyCount{}, Status: map[string]int{}, ByDept: []DeptIssueRow{}}
	rows, err := s.pool.Query(ctx, `select to_char(d.day,'MM-DD'),
		(select count(*) from issue where created_at::date = d.day),
		(select count(*) from issue where closed_at::date = d.day)
		from generate_series(current_date - ($1::int - 1), current_date, interval '1 day') d(day)
		order by d.day`, days)
	if err != nil {
		return nil, wrapErr("统计隐患趋势", err)
	}
	defer rows.Close()
	for rows.Next() {
		var d DailyCount
		if err := rows.Scan(&d.Date, &d.Reported, &d.Closed); err != nil {
			return nil, err
		}
		rep.Daily = append(rep.Daily, d)
	}
	st, err := s.pool.Query(ctx, `select status, count(*) from issue group by status`)
	if err != nil {
		return nil, wrapErr("统计隐患状态", err)
	}
	defer st.Close()
	for st.Next() {
		var k string
		var n int
		if err := st.Scan(&k, &n); err != nil {
			return nil, err
		}
		rep.Status[k] = n
		rep.Total += n
		if k == model.IssueClosed {
			rep.Closed = n
		}
	}
	_ = s.pool.QueryRow(ctx, `select count(*) from issue where overdue and status<>'closed'`).Scan(&rep.Overdue)
	_ = s.pool.QueryRow(ctx, `select coalesce(avg(extract(epoch from closed_at-created_at)/3600),0)
		from issue where closed_at is not null`).Scan(&rep.AvgCloseHr)
	dep, err := s.pool.Query(ctx, `select coalesce(d.name,'未分配'), count(i.id),
		count(i.id) filter (where i.status='closed'),
		count(i.id) filter (where i.status<>'closed')
		from issue i left join org_dept d on d.id=i.dept_id
		group by 1 order by 2 desc`)
	if err != nil {
		return nil, wrapErr("统计部门隐患", err)
	}
	defer dep.Close()
	for dep.Next() {
		var r DeptIssueRow
		if err := dep.Scan(&r.Dept, &r.Total, &r.Closed, &r.Open); err != nil {
			return nil, err
		}
		rep.ByDept = append(rep.ByDept, r)
	}
	return rep, nil
}
