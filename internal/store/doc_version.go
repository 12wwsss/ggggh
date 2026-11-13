package store

import (
	"context"
	"encoding/json"
	"time"

	"coalrag/internal/model"
)

// DocVersion 是文档的一次发布快照。
type DocVersion struct {
	ID         int64     `json:"id"`
	DocID      int64     `json:"doc_id"`
	Version    int       `json:"version"`
	Title      string    `json:"title"`
	ChunkCount int       `json:"chunk_count"`
	OperatorID *int64    `json:"operator_id,omitempty"`
	Operator   string    `json:"operator,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// SaveDocVersion 保存一次发布快照；同一版本重复发布时覆盖。
func (s *Store) SaveDocVersion(ctx context.Context, docID int64, version int, title string,
	operatorID *int64, chunks []model.Chunk) error {

	type snap struct {
		ClauseNo string `json:"clause_no"`
		Chapter  string `json:"chapter"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		Part     int    `json:"part"`
	}
	items := make([]snap, 0, len(chunks))
	for _, c := range chunks {
		items = append(items, snap{ClauseNo: c.ClauseNo, Chapter: c.Chapter, Title: c.Title, Content: c.Content, Part: c.Part})
	}
	blob, err := json.Marshal(items)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `insert into doc_version(doc_id, version, title, chunk_count, operator_id, snapshot)
		values($1,$2,$3,$4,$5,$6)
		on conflict (doc_id, version) do update set title=excluded.title, chunk_count=excluded.chunk_count,
			operator_id=excluded.operator_id, snapshot=excluded.snapshot, created_at=now()`,
		docID, version, title, len(items), operatorID, blob)
	return wrapErr("保存文档版本", err)
}

// ListDocVersions 列出快照（不含正文）。
func (s *Store) ListDocVersions(ctx context.Context, docID int64) ([]DocVersion, error) {
	rows, err := s.pool.Query(ctx, `select v.id, v.doc_id, v.version, v.title, v.chunk_count,
		v.operator_id, coalesce(u.name,''), v.created_at
		from doc_version v left join org_user u on u.id=v.operator_id
		where v.doc_id=$1 order by v.version desc`, docID)
	if err != nil {
		return nil, wrapErr("查询文档版本", err)
	}
	defer rows.Close()

	out := []DocVersion{}
	for rows.Next() {
		var v DocVersion
		if err := rows.Scan(&v.ID, &v.DocID, &v.Version, &v.Title, &v.ChunkCount,
			&v.OperatorID, &v.Operator, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// LoadDocVersion 读取某个版本的切片内容。
func (s *Store) LoadDocVersion(ctx context.Context, docID int64, version int) ([]model.Chunk, *DocVersion, error) {
	var v DocVersion
	var blob []byte
	err := s.pool.QueryRow(ctx, `select id, doc_id, version, title, chunk_count, operator_id, created_at, snapshot
		from doc_version where doc_id=$1 and version=$2`, docID, version).
		Scan(&v.ID, &v.DocID, &v.Version, &v.Title, &v.ChunkCount, &v.OperatorID, &v.CreatedAt, &blob)
	if isNoRows(err) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, wrapErr("读取文档版本", err)
	}

	var items []struct {
		ClauseNo string `json:"clause_no"`
		Chapter  string `json:"chapter"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		Part     int    `json:"part"`
	}
	if err := json.Unmarshal(blob, &items); err != nil {
		return nil, nil, wrapErr("解析版本快照", err)
	}
	chunks := make([]model.Chunk, 0, len(items))
	for i, it := range items {
		chunks = append(chunks, model.Chunk{
			DocID: docID, ClauseNo: it.ClauseNo, Chapter: it.Chapter, Title: it.Title,
			Content: it.Content, Part: it.Part, Seq: i + 1, CharCount: len([]rune(it.Content)),
			Published: true,
		})
	}
	return chunks, &v, nil
}

// RestoreChunks 用历史快照替换当前切片，并把文档置为已发布、版本号加一。
func (s *Store) RestoreChunks(ctx context.Context, docID int64, chunks []model.Chunk) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `delete from doc_chunk where doc_id=$1`, docID); err != nil {
		return 0, wrapErr("清理当前切片", err)
	}
	for i := range chunks {
		c := &chunks[i]
		if _, err := tx.Exec(ctx, `insert into doc_chunk
			(doc_id, clause_no, chapter, seq, title, content, char_count, part, published)
			values ($1,nullif($2,''),nullif($3,''),$4,nullif($5,''),$6,$7,$8,true)`,
			docID, c.ClauseNo, c.Chapter, i+1, c.Title, c.Content, len([]rune(c.Content)), c.Part); err != nil {
			return 0, wrapErr("写入回滚切片", err)
		}
	}
	var version int
	if err := tx.QueryRow(ctx, `update doc set status='published', chunk_count=$2, version=version+1,
		published_at=now(), updated_at=now(), error_msg=null where id=$1 returning version`,
		docID, len(chunks)).Scan(&version); err != nil {
		return 0, wrapErr("更新文档版本号", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return version, nil
}
