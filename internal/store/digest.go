package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// IssueDigest 是一段时间内的问题汇总，用于日报/周报推送。
type IssueDigest struct {
	Period        string         `json:"period"`
	From          string         `json:"from"`
	To            string         `json:"to"`
	Reported      int            `json:"reported"`
	Closed        int            `json:"closed"`
	OpenTotal     int            `json:"open_total"`
	Overdue       int            `json:"overdue"`
	AvgCloseHours float64        `json:"avg_close_hours"`
	ByDept        []DeptIssueRow `json:"by_dept"`
	ByPost        []PostIssueRow `json:"by_post"`
	ByCate        []CateIssueRow `json:"by_cate"`
}

// CateIssueRow 问题分类分布。
type CateIssueRow struct {
	Name  string `json:"name"`
	Total int    `json:"total"`
}

// IssueDigest 统计 [from, to) 区间内新增/闭环数，以及当前未闭环、超时、区队/岗位/分类分布。
func (s *Store) IssueDigest(ctx context.Context, from, to time.Time) (*IssueDigest, error) {
	d := &IssueDigest{
		From:   from.Format("2006-01-02"),
		To:     to.Add(-time.Second).Format("2006-01-02"),
		ByDept: []DeptIssueRow{},
		ByPost: []PostIssueRow{},
		ByCate: []CateIssueRow{},
	}
	err := s.pool.QueryRow(ctx, `select
		count(*) filter (where created_at >= $1 and created_at < $2),
		count(*) filter (where closed_at >= $1 and closed_at < $2),
		count(*) filter (where status <> 'closed'),
		count(*) filter (where overdue and status <> 'closed'),
		coalesce(avg(extract(epoch from closed_at-created_at)/3600)
		         filter (where closed_at >= $1 and closed_at < $2), 0)
		from issue`, from, to).Scan(&d.Reported, &d.Closed, &d.OpenTotal, &d.Overdue, &d.AvgCloseHours)
	if err != nil {
		return nil, wrapErr("统计问题汇总", err)
	}

	rows, err := s.pool.Query(ctx, `select coalesce(d.name,'未分配'), count(*),
		count(*) filter (where i.status='closed'),
		count(*) filter (where i.status<>'closed')
		from issue i left join org_dept d on d.id=i.dept_id
		group by 1 order by 2 desc limit 5`)
	if err != nil {
		return nil, wrapErr("统计区队分布", err)
	}
	for rows.Next() {
		var r DeptIssueRow
		if err := rows.Scan(&r.Dept, &r.Total, &r.Closed, &r.Open); err != nil {
			rows.Close()
			return nil, err
		}
		d.ByDept = append(d.ByDept, r)
	}
	rows.Close()

	prows, err := s.pool.Query(ctx, `select coalesce(nullif(i.post,''),'未填岗位'), count(*),
		count(*) filter (where i.status='closed'),
		count(*) filter (where i.status<>'closed')
		from issue i group by 1 order by 2 desc limit 5`)
	if err != nil {
		return nil, wrapErr("统计岗位分布", err)
	}
	for prows.Next() {
		var r PostIssueRow
		if err := prows.Scan(&r.Post, &r.Total, &r.Closed, &r.Open); err != nil {
			prows.Close()
			return nil, err
		}
		d.ByPost = append(d.ByPost, r)
	}
	prows.Close()

	crows, err := s.pool.Query(ctx, `select coalesce(c.name,'未分类'), count(*)
		from issue i left join issue_cate c on c.id=i.cate_id
		group by 1 order by 2 desc limit 5`)
	if err != nil {
		return nil, wrapErr("统计分类分布", err)
	}
	for crows.Next() {
		var r CateIssueRow
		if err := crows.Scan(&r.Name, &r.Total); err != nil {
			crows.Close()
			return nil, err
		}
		d.ByCate = append(d.ByCate, r)
	}
	crows.Close()
	return d, nil
}

// DigestText 把汇总压成一段适合站内消息的正文。
func (d *IssueDigest) DigestText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s 至 %s\n", d.From, d.To)
	fmt.Fprintf(&b, "新增 %d 条，闭环 %d 条，平均闭环 %.1f 小时\n", d.Reported, d.Closed, d.AvgCloseHours)
	fmt.Fprintf(&b, "当前未闭环 %d 条，其中超时 %d 条\n", d.OpenTotal, d.Overdue)
	if len(d.ByDept) > 0 {
		parts := make([]string, 0, 3)
		for i, r := range d.ByDept {
			if i == 3 {
				break
			}
			parts = append(parts, fmt.Sprintf("%s %d", r.Dept, r.Total))
		}
		fmt.Fprintf(&b, "区队 Top：%s\n", strings.Join(parts, "、"))
	}
	if len(d.ByPost) > 0 {
		parts := make([]string, 0, 3)
		for i, r := range d.ByPost {
			if i == 3 {
				break
			}
			parts = append(parts, fmt.Sprintf("%s %d", r.Post, r.Total))
		}
		fmt.Fprintf(&b, "岗位 Top：%s\n", strings.Join(parts, "、"))
	}
	if len(d.ByCate) > 0 {
		parts := make([]string, 0, 3)
		for i, r := range d.ByCate {
			if i == 3 {
				break
			}
			parts = append(parts, fmt.Sprintf("%s %d", r.Name, r.Total))
		}
		fmt.Fprintf(&b, "分类 Top：%s", strings.Join(parts, "、"))
	}
	return strings.TrimRight(b.String(), "\n")
}

// UsersWithRoles 返回指定角色的在册用户，用于按角色推送消息。
func (s *Store) UsersWithRoles(ctx context.Context, roles ...string) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `select id from org_user
		where status='active' and role = any($1) order by id`, roles)
	if err != nil {
		return nil, wrapErr("查询角色用户", err)
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// HasTaskToday 判断今天是否已投递过同类任务，避免日报/周报重复生成。
func (s *Store) HasTaskToday(ctx context.Context, kind string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from task
		where kind=$1 and created_at >= current_date`, kind).Scan(&n)
	return n > 0, wrapErr("检查今日任务", err)
}
