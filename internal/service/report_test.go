package service

import (
	"strings"
	"testing"
	"time"

	"coalrag/internal/store"
)

func TestDigestWindow(t *testing.T) {
	now := time.Date(2026, 11, 26, 8, 3, 0, 0, time.Local) // 周四上午 8:03

	from, end := digestWindow("daily", now)
	if end.Format("2006-01-02 15:04") != "2026-11-26 00:00" {
		t.Errorf("日报窗口右端 = %s，期望今天 00:00", end)
	}
	if from.Format("2006-01-02 15:04") != "2026-11-25 00:00" {
		t.Errorf("日报窗口左端 = %s，期望昨天 00:00", from)
	}

	from, end = digestWindow("weekly", now)
	if gap := end.Sub(from).Hours() / 24; gap != 6 {
		t.Errorf("周报窗口跨 %.0f 天，期望 6 天（含今天共 7 天）", gap)
	}
	if from.Format("2006-01-02") != "2026-11-20" {
		t.Errorf("周报窗口左端 = %s，期望 11-20", from.Format("2006-01-02"))
	}
}

func TestIssueDigestTitle(t *testing.T) {
	d := &store.IssueDigest{Period: "daily", Reported: 3, Closed: 2}
	if got := IssueDigestTitle(d); !strings.Contains(got, "隐患日报") || !strings.Contains(got, "新增 3") {
		t.Errorf("日报标题 = %q", got)
	}
	d.Period = "weekly"
	if got := IssueDigestTitle(d); !strings.Contains(got, "隐患周报") {
		t.Errorf("周报标题 = %q", got)
	}
}
