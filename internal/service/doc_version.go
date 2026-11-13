package service

import (
	"context"
	"strings"

	"coalrag/internal/model"
)

// RollbackDoc 把文档回滚到历史版本：替换切片、版本号加一、重新生成向量。
func (s *Service) RollbackDoc(ctx context.Context, docID int64, version int, operatorID *int64) (*model.Doc, error) {
	chunks, _, err := s.Store.LoadDocVersion(ctx, docID, version)
	if err != nil {
		return nil, err
	}
	newVersion, err := s.Store.RestoreChunks(ctx, docID, chunks)
	if err != nil {
		return nil, err
	}
	// 回滚会重建切片（chunk_id 变化），旧向量必须先摘掉，
	// 否则在新向量生成完成前，引用会指向已删除的切片。
	if err := s.Vector.DeleteByDoc(ctx, docID); err != nil {
		s.Log.Error("清理旧向量失败", "err", err, "doc_id", docID)
	}
	if err := s.RebuildLexical(ctx); err != nil {
		s.Log.Error("重建词法索引失败", "err", err, "doc_id", docID)
	}
	doc, err := s.Store.GetDoc(ctx, docID)
	if err != nil {
		return nil, err
	}
	if err := s.Store.SaveDocVersion(ctx, docID, newVersion, doc.Title, operatorID, chunks); err != nil {
		return nil, err
	}
	if _, err := s.Store.Enqueue(ctx, TaskDocEmbed, map[string]any{"doc_id": docID}, 0); err != nil {
		return nil, err
	}
	s.Log.Info("文档已回滚", "doc_id", docID, "from_version", version, "new_version", newVersion)
	return doc, nil
}

// SnapshotDoc 在发布后保存快照；失败只记日志，不影响发布本身。
func (s *Service) SnapshotDoc(ctx context.Context, docID int64, operatorID *int64) {
	doc, err := s.Store.GetDoc(ctx, docID)
	if err != nil {
		s.Log.Error("读取文档失败，跳过版本快照", "doc_id", docID, "err", err)
		return
	}
	chunks, err := s.Store.ListChunks(ctx, docID, true)
	if err != nil {
		s.Log.Error("读取切片失败，跳过版本快照", "doc_id", docID, "err", err)
		return
	}
	if err := s.Store.SaveDocVersion(ctx, docID, doc.Version, doc.Title, operatorID, chunks); err != nil {
		s.Log.Error("保存版本快照失败", "doc_id", docID, "err", err)
	}
}

// ClauseDiff 是两个版本之间的条款差异。
type ClauseDiff struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
	Same    int      `json:"same"`
}

// DiffVersions 对比两个版本的条款集合：按条款号对齐，正文不同记为修改。
func (s *Service) DiffVersions(ctx context.Context, docID int64, from, to int) (*ClauseDiff, error) {
	oldChunks, _, err := s.Store.LoadDocVersion(ctx, docID, from)
	if err != nil {
		return nil, err
	}
	newChunks, _, err := s.Store.LoadDocVersion(ctx, docID, to)
	if err != nil {
		return nil, err
	}

	type entry struct {
		clause  string
		content string
	}
	toMap := func(list []model.Chunk) map[string]entry {
		m := make(map[string]entry, len(list))
		for _, c := range list {
			key := c.ClauseNo
			if key == "" {
				key = "（无条款号）" + prefixRunes(c.Content, 12)
			}
			m[key] = entry{clause: c.ClauseNo, content: c.Content}
		}
		return m
	}

	oldM, newM := toMap(oldChunks), toMap(newChunks)
	diff := &ClauseDiff{Added: []string{}, Removed: []string{}, Changed: []string{}}
	for key, n := range newM {
		o, ok := oldM[key]
		switch {
		case !ok:
			diff.Added = append(diff.Added, label(n.clause, key))
		case strings.TrimSpace(o.content) != strings.TrimSpace(n.content):
			diff.Changed = append(diff.Changed, label(n.clause, key))
		default:
			diff.Same++
		}
	}
	for key, o := range oldM {
		if _, ok := newM[key]; !ok {
			diff.Removed = append(diff.Removed, label(o.clause, key))
		}
	}
	return diff, nil
}

func label(clause, fallback string) string {
	if strings.TrimSpace(clause) != "" {
		return clause
	}
	return fallback
}

// prefixRunes 按字符截断，避免切坏多字节字符。
func prefixRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
