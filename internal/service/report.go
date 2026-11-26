package service

import (
	"context"
	"fmt"
	"time"

	"coalrag/internal/store"
)

// IssueDigest 按周期汇总问题数据：daily=昨天一整天，weekly=最近 7 天（含今天）。
// 窗口统一按服务器本地时区对齐到 00:00，避免跨时区统计错位。
func (s *Service) IssueDigest(ctx context.Context, period string, now time.Time) (*store.IssueDigest, error) {
	from, end := digestWindow(period, now)
	if period != "weekly" {
		period = "daily"
	}
	d, err := s.Store.IssueDigest(ctx, from, end)
	if err != nil {
		return nil, err
	}
	d.Period = period
	return d, nil
}

// digestWindow 返回 [from, end) 的半开区间：日报=昨天 00:00 到今天 00:00，周报=最近 7 天。
func digestWindow(period string, now time.Time) (from, end time.Time) {
	loc := now.Location()
	end = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if period == "weekly" {
		return end.AddDate(0, 0, -6), end
	}
	return end.AddDate(0, 0, -1), end
}

// IssueDigestTitle 生成消息标题，例如「隐患日报：新增 3 / 闭环 2」。
func IssueDigestTitle(d *store.IssueDigest) string {
	label := "日报"
	if d.Period == "weekly" {
		label = "周报"
	}
	return fmt.Sprintf("隐患%s：新增 %d / 闭环 %d", label, d.Reported, d.Closed)
}
