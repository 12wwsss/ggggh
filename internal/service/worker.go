package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"coalrag/internal/model"
)

// Worker 轮询数据库任务表并执行任务；不依赖 Redis 或消息队列。
type Worker struct {
	Svc      *Service
	Interval time.Duration
}

// Run 阻塞运行直到 ctx 取消。
func (w *Worker) Run(ctx context.Context) {
	if w.Interval <= 0 {
		w.Interval = time.Second
	}
	tk := time.NewTicker(w.Interval)
	defer tk.Stop()

	sweep := time.NewTicker(5 * time.Minute)
	defer sweep.Stop()

	// 任务表清理每天一次即可，独立节拍，避免和扫描任务挤在一起。
	retention := time.NewTicker(24 * time.Hour)
	defer retention.Stop()

	// 启动时先恢复僵死任务，并立刻做一次超时扫描。
	if n, err := w.Svc.Store.RecoverStaleTasks(ctx, 10*time.Minute); err != nil {
		w.Svc.Log.Error("恢复僵死任务失败", "err", err)
	} else if n > 0 {
		w.Svc.Log.Info("恢复僵死任务", "count", n)
	}
	w.enqueueSweeps(ctx)
	w.enqueueRetention(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sweep.C:
			w.enqueueSweeps(ctx)
		case <-retention.C:
			w.enqueueRetention(ctx)
		case <-tk.C:
			w.processOnce(ctx)
		}
	}
}

func (w *Worker) enqueueSweeps(ctx context.Context) {
	for _, kind := range []string{TaskIssueSweep, TaskBroadcastScan, TaskExamTimeout} {
		if _, err := w.Svc.Store.Enqueue(ctx, kind, map[string]any{}, 0); err != nil {
			w.Svc.Log.Error("投递扫描任务失败", "kind", kind, "err", err)
		}
	}
}

func (w *Worker) enqueueRetention(ctx context.Context) {
	if _, err := w.Svc.Store.Enqueue(ctx, TaskRetention, map[string]any{}, 0); err != nil {
		w.Svc.Log.Error("投递任务清理失败", "err", err)
	}
}

// processOnce 领取并执行一批任务。
func (w *Worker) processOnce(ctx context.Context) {
	tasks, err := w.Svc.Store.ClaimTasks(ctx, 5)
	if err != nil {
		w.Svc.Log.Error("领取任务失败", "err", err)
		return
	}
	for _, t := range tasks {
		t := t
		if err := w.handle(ctx, &t); err != nil {
			w.Svc.Log.Error("任务执行失败", "task_id", t.ID, "kind", t.Kind, "err", err)
			if ferr := w.Svc.Store.FailTask(ctx, t.ID, err.Error(), time.Duration(t.Attempts)*10*time.Second); ferr != nil {
				w.Svc.Log.Error("更新任务状态失败", "task_id", t.ID, "err", ferr)
			}
			continue
		}
		if err := w.Svc.Store.FinishTask(ctx, t.ID); err != nil {
			w.Svc.Log.Error("标记任务完成失败", "task_id", t.ID, "err", err)
		}
	}
}

func (w *Worker) handle(ctx context.Context, t *model.Task) error {
	docID, err := payloadInt64(t.Payload, "doc_id")
	switch t.Kind {
	case TaskDocParse:
		if err != nil {
			return fmt.Errorf("任务载荷缺少 doc_id: %w", err)
		}
		return w.Svc.ParseDoc(ctx, docID)
	case TaskDocEmbed:
		if err != nil {
			return fmt.Errorf("任务载荷缺少 doc_id: %w", err)
		}
		return w.Svc.EmbedDoc(ctx, docID)
	case TaskIssueSweep:
		ids, err := w.Svc.Store.SweepOverdueIssues(ctx)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			w.Svc.Log.Warn("问题超时", "issue_ids", ids, "count", len(ids))
		}
		return nil
	case TaskBroadcastScan:
		n, err := w.Svc.Store.PendingBroadcastCount(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			w.Svc.Log.Info("宣贯任务待签收", "pending", n)
		}
		return nil
	case TaskExamTimeout:
		ids, err := w.Svc.Store.SweepExamTimeouts(ctx)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			w.Svc.Log.Warn("考试超时自动交卷", "record_ids", ids, "count", len(ids))
		}
		return nil
	case TaskRetention:
		days := w.Svc.Cfg.Task.RetentionDays
		if days <= 0 {
			days = 7
		}
		n, err := w.Svc.Store.PurgeFinishedTasks(ctx, time.Duration(days)*24*time.Hour)
		if err != nil {
			return err
		}
		if n > 0 {
			w.Svc.Log.Info("清理历史任务", "removed", n, "retention_days", days)
		}
		return nil
	default:
		return fmt.Errorf("未知任务类型 %s", t.Kind)
	}
}

func payloadInt64(payload json.RawMessage, key string) (int64, error) {
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return 0, err
	}
	v, ok := m[key]
	if !ok {
		return 0, fmt.Errorf("缺少字段 %s", key)
	}
	switch n := v.(type) {
	case float64:
		return int64(n), nil
	case json.Number:
		return n.Int64()
	default:
		return 0, fmt.Errorf("字段 %s 类型异常", key)
	}
}
