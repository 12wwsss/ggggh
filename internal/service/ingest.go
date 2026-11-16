// Package service 承载业务编排：文档入库、检索、问答、评测、任务处理。
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"coalrag/internal/config"
	"coalrag/internal/llm"
	"coalrag/internal/model"
	"coalrag/internal/parser"
	"coalrag/internal/search"
	"coalrag/internal/store"
	"coalrag/internal/vector"
)

// 任务类型。
const (
	TaskDocParse      = "doc_parse"
	TaskDocEmbed      = "doc_embed"
	TaskIssueSweep    = "issue_overdue_sweep"
	TaskBroadcastScan = "broadcast_deadline_scan"
	TaskExamTimeout   = "exam_timeout_sweep"
	TaskRetention     = "task_retention"
	TaskLogRetention  = "log_retention"
)

// Service 是业务编排入口。
type Service struct {
	Store    *store.Store
	Cfg      *config.Config
	Chat     llm.Chat
	Embedder llm.Embedder
	Vector   vector.Store
	Lexical  *search.Index
	Log      *slog.Logger
}

// UploadDir 返回上传目录。
func (s *Service) UploadDir() string { return filepath.Join(s.Cfg.StorageDir, "uploads") }

// EnsureDirs 创建存储目录。
func (s *Service) EnsureDirs() error {
	for _, d := range []string{s.UploadDir(), filepath.Join(s.Cfg.StorageDir, "issues"), filepath.Join(s.Cfg.StorageDir, "eval")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("创建目录 %s: %w", d, err)
		}
	}
	return nil
}

// UploadDoc 保存文件并登记文档，随后投递解析任务。
//
// 按内容 sha256 去重：同一份文件重复上传时直接复用已有文档（解析失败的除外），
// 避免同一条款出现多份副本抢检索排名；force=true 可强制重新入库。
func (s *Service) UploadDoc(ctx context.Context, filename string, src io.Reader, size int64,
	title, category string, uploaderID int64, force bool) (*model.Doc, bool, error) {

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return nil, false, fmt.Errorf("文件缺少扩展名")
	}
	allowed := map[string]bool{".docx": true, ".doc": true, ".pdf": true, ".txt": true, ".md": true, ".wps": true}
	if !allowed[ext] {
		return nil, false, fmt.Errorf("不支持的文件类型 %s", ext)
	}
	if title == "" {
		title = strings.TrimSuffix(filename, ext)
	}
	if category == "" {
		category = "规程"
	}
	if err := os.MkdirAll(s.UploadDir(), 0o755); err != nil {
		return nil, false, err
	}

	// 先落到临时文件并计算哈希，查重通过后再改名为正式文件。
	tmp, err := os.CreateTemp(s.UploadDir(), "upload-*"+ext)
	if err != nil {
		return nil, false, fmt.Errorf("创建临时文件: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(tmp, h), src)
	closeErr := tmp.Close()
	if copyErr != nil {
		cleanup()
		return nil, false, fmt.Errorf("写入上传文件: %w", copyErr)
	}
	if closeErr != nil {
		cleanup()
		return nil, false, closeErr
	}
	sha := hex.EncodeToString(h.Sum(nil))

	if !force {
		if existing, err := s.Store.FindDocBySHA(ctx, sha); err == nil && existing != nil {
			cleanup()
			s.Log.Info("上传去重：已存在相同内容", "doc_id", existing.ID, "sha256", sha[:12])
			return existing, true, nil
		}
	}

	// 文件名带内容哈希前缀，强制重新上传也不会互相覆盖。
	name := fmt.Sprintf("%d_%s_%s%s", uploaderID, sanitize(strings.TrimSuffix(filename, ext)), sha[:8], ext)
	path := filepath.Join(s.UploadDir(), name)
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return nil, false, fmt.Errorf("保存上传文件: %w", err)
	}

	doc := &model.Doc{
		Title:      title,
		Category:   category,
		SourceName: path,
		FileSize:   n,
		SHA256:     sha,
		Status:     model.DocParsing,
		UploaderID: &uploaderID,
	}
	id, err := s.Store.CreateDoc(ctx, doc)
	if err != nil {
		return nil, false, err
	}
	doc.ID = id

	if _, err := s.Store.Enqueue(ctx, TaskDocParse, map[string]any{"doc_id": id}, 0); err != nil {
		return nil, false, err
	}
	saved, err := s.Store.GetDoc(ctx, id)
	return saved, false, err
}

// ParseDoc 解析并按条款切分。解析在内存完成后一次性写入，
// 失败则整体回滚并把文档置为 failed，不留脏切片。
func (s *Service) ParseDoc(ctx context.Context, docID int64) error {
	doc, err := s.Store.GetDoc(ctx, docID)
	if err != nil {
		return err
	}
	if doc.Status == model.DocFailed {
		// 允许重试已失败的文档
		_ = s.Store.UpdateDocStatus(ctx, docID, model.DocParsing, "")
	}

	p, err := parser.Pick(doc.SourceName)
	if err != nil {
		_ = s.Store.UpdateDocStatus(ctx, docID, model.DocFailed, err.Error())
		return err
	}
	blocks, err := p.Parse(ctx, doc.SourceName)
	if err != nil {
		_ = s.Store.UpdateDocStatus(ctx, docID, model.DocFailed, "解析失败: "+err.Error())
		return err
	}
	chunks, err := parser.Split(blocks)
	if err != nil {
		_ = s.Store.UpdateDocStatus(ctx, docID, model.DocFailed, "切分失败: "+err.Error())
		return err
	}
	for i := range chunks {
		chunks[i].DocID = docID
	}
	if err := s.Store.ReplaceChunks(ctx, docID, chunks); err != nil {
		_ = s.Store.UpdateDocStatus(ctx, docID, model.DocFailed, "入库失败: "+err.Error())
		return err
	}
	// 切片重建后旧的 chunk_id 已失效，先把该文档从内存索引里摘掉，
	// 避免重新发布前检索到已删除的切片（引用会指向不存在的原文）。
	if err := s.Vector.DeleteByDoc(ctx, docID); err != nil {
		s.Log.Error("清理旧向量失败", "err", err, "doc_id", docID)
	}
	if err := s.RebuildLexical(ctx); err != nil {
		s.Log.Error("重建词法索引失败", "err", err, "doc_id", docID)
	}
	if err := s.Store.UpdateDocStatus(ctx, docID, model.DocPendingReview, ""); err != nil {
		return err
	}
	s.Log.Info("文档解析完成", "doc_id", docID, "parser", p.Name(), "chunks", len(chunks))
	return nil
}

// PublishDoc 发布文档、保存版本快照并投递向量任务。
func (s *Service) PublishDoc(ctx context.Context, docID int64, operatorID *int64) (*model.Doc, error) {
	if err := s.Store.PublishDoc(ctx, docID); err != nil {
		return nil, err
	}
	s.SnapshotDoc(ctx, docID, operatorID)
	if _, err := s.Store.Enqueue(ctx, TaskDocEmbed, map[string]any{"doc_id": docID}, 0); err != nil {
		return nil, err
	}
	return s.Store.GetDoc(ctx, docID)
}

// EmbedDoc 为已发布切片生成向量并写入索引。
func (s *Service) EmbedDoc(ctx context.Context, docID int64) error {
	chunks, err := s.Store.ListChunks(ctx, docID, true)
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return fmt.Errorf("文档 %d 没有已发布切片", docID)
	}
	doc, err := s.Store.GetDoc(ctx, docID)
	if err != nil {
		return err
	}

	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = embedText(c)
	}
	vecs, err := s.Embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("生成向量失败: %w", err)
	}
	if len(vecs) != len(chunks) {
		return fmt.Errorf("向量数量 %d 与切片数量 %d 不一致", len(vecs), len(chunks))
	}

	docs := make([]vector.Doc, 0, len(chunks))
	ids := make([]int64, 0, len(chunks))
	for i, c := range chunks {
		if len(vecs[i]) != s.Cfg.Embed.Dim {
			return fmt.Errorf("向量维度 %d 与配置 %d 不一致", len(vecs[i]), s.Cfg.Embed.Dim)
		}
		docs = append(docs, vector.Doc{
			ChunkID:  c.ID,
			DocID:    c.DocID,
			DocTitle: doc.Title,
			ClauseNo: c.ClauseNo,
			Chapter:  c.Chapter,
			Content:  c.Content,
			Seq:      c.Seq,
			Vec:      vector.Normalize(vecs[i]),
		})
		ids = append(ids, c.ID)
	}

	// pgvector 实现自行落库；内存实现的 persist 回调也已写入。
	if err := s.Vector.Upsert(ctx, docs); err != nil {
		return fmt.Errorf("写入向量索引: %w", err)
	}
	_ = ids

	// 新切片进词法索引：整表重建，避免旧切片（ID 已变）残留。
	if err := s.RebuildLexical(ctx); err != nil {
		s.Log.Error("重建词法索引失败", "err", err, "doc_id", docID)
	}
	s.Log.Info("向量写入完成", "doc_id", docID, "chunks", len(docs), "impl", s.Vector.Kind())
	return nil
}

func embedText(c model.Chunk) string {
	parts := make([]string, 0, 3)
	if c.Chapter != "" {
		parts = append(parts, c.Chapter)
	}
	if c.ClauseNo != "" {
		parts = append(parts, c.ClauseNo)
	}
	parts = append(parts, c.Content)
	return strings.Join(parts, " ")
}

func sanitize(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r > 127:
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len([]rune(out)) > 40 {
		out = string([]rune(out)[:40])
	}
	if out == "" {
		out = "doc"
	}
	return out
}

// ReloadIndex 重建向量与词法索引。
func (s *Service) ReloadIndex(ctx context.Context) error {
	if err := s.Vector.Reload(ctx); err != nil {
		return err
	}
	return s.RebuildLexical(ctx)
}

// RebuildLexical 用已发布切片重建 BM25 索引。
// 发布动作不频繁，整表重建比增量维护简单，也不会留下旧切片的残留。
func (s *Service) RebuildLexical(ctx context.Context) error {
	if s.Lexical == nil {
		return nil
	}
	chunks, err := s.Store.ListPublishedChunks(ctx)
	if err != nil {
		return err
	}
	s.Lexical.Clear()
	for _, c := range chunks {
		s.Lexical.Add(c.ID, strings.Join([]string{c.Chapter, c.ClauseNo, c.Content}, " "))
	}
	s.Log.Info("词法索引就绪", "chunks", s.Lexical.Size())
	return nil
}
