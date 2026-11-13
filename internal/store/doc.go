package store

import (
	"context"
	"fmt"

	"coalrag/internal/model"
)

const sqlDocCols = `d.id, d.title, d.category, coalesce(d.source_name,''), d.file_size,
	coalesce(d.sha256,''), d.status, d.version, d.chunk_count, d.uploader_id,
	coalesce(u.name,''), coalesce(d.error_msg,''), d.created_at, d.updated_at, d.published_at`

const sqlDocFrom = ` from doc d left join org_user u on u.id = d.uploader_id `

// CreateDoc 写入文档元数据。
func (s *Store) CreateDoc(ctx context.Context, d *model.Doc) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `insert into doc
		(title, category, source_name, stored_path, file_size, sha256, status, uploader_id, error_msg)
		values ($1,$2,$3,$4,$5,$6,$7,$8,nullif($9,'')) returning id`,
		d.Title, d.Category, d.SourceName, d.SourceName, d.FileSize, d.SHA256, d.Status, d.UploaderID, d.ErrorMsg).Scan(&id)
	return id, wrapErr("写入文档", err)
}

// GetDoc 按主键取文档。
func (s *Store) GetDoc(ctx context.Context, id int64) (*model.Doc, error) {
	row := s.pool.QueryRow(ctx, `select `+sqlDocCols+sqlDocFrom+` where d.id=$1`, id)
	var d model.Doc
	err := row.Scan(&d.ID, &d.Title, &d.Category, &d.SourceName, &d.FileSize, &d.SHA256,
		&d.Status, &d.Version, &d.ChunkCount, &d.UploaderID, &d.Uploader, &d.ErrorMsg,
		&d.CreatedAt, &d.UpdatedAt, &d.PublishedAt)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询文档", err)
	}
	return &d, nil
}

// ListDocs 分页查询文档。
func (s *Store) ListDocs(ctx context.Context, status, keyword string, limit, offset int) ([]model.Doc, int, error) {
	q := `select ` + sqlDocCols + `, count(*) over() as total` + sqlDocFrom +
		` where ($1 = '' or d.status = $1) and ($2 = '' or d.title ilike '%'||$2||'%')
		  order by d.id desc limit $3 offset $4`
	rows, err := s.pool.Query(ctx, q, status, keyword, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询文档列表", err)
	}
	defer rows.Close()
	var out []model.Doc
	total := 0
	for rows.Next() {
		var d model.Doc
		if err := rows.Scan(&d.ID, &d.Title, &d.Category, &d.SourceName, &d.FileSize, &d.SHA256,
			&d.Status, &d.Version, &d.ChunkCount, &d.UploaderID, &d.Uploader, &d.ErrorMsg,
			&d.CreatedAt, &d.UpdatedAt, &d.PublishedAt, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, rows.Err()
}

// UpdateDocStatus 更新文档状态。
func (s *Store) UpdateDocStatus(ctx context.Context, id int64, status, errMsg string) error {
	tag, err := s.pool.Exec(ctx, `update doc set status=$2, error_msg=nullif($3,''), updated_at=now()
		where id=$1`, id, status, errMsg)
	if err != nil {
		return wrapErr("更新文档状态", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteDoc 删除文档（切片与向量随外键级联删除）。
func (s *Store) DeleteDoc(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `delete from doc where id=$1`, id)
	if err != nil {
		return wrapErr("删除文档", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ReplaceChunks 在一个事务内替换某文档的全部切片：先删后插，
// 解析失败时整体回滚，保证不写脏数据。
func (s *Store) ReplaceChunks(ctx context.Context, docID int64, chunks []model.Chunk) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return wrapErr("开启事务", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `delete from doc_chunk where doc_id=$1`, docID); err != nil {
		return wrapErr("清理旧切片", err)
	}
	for i := range chunks {
		c := &chunks[i]
		if _, err := tx.Exec(ctx, `insert into doc_chunk
			(doc_id, clause_no, chapter, seq, title, content, char_count, part, published)
			values ($1,nullif($2,''),nullif($3,''),$4,nullif($5,''),$6,$7,$8,false)`,
			docID, c.ClauseNo, c.Chapter, i+1, c.Title, c.Content, len([]rune(c.Content)), c.Part); err != nil {
			return wrapErr("写入切片", err)
		}
	}
	if _, err := tx.Exec(ctx, `update doc set chunk_count=$2, updated_at=now() where id=$1`,
		docID, len(chunks)); err != nil {
		return wrapErr("更新切片数", err)
	}
	return tx.Commit(ctx)
}

// ListChunks 列出文档切片。
func (s *Store) ListChunks(ctx context.Context, docID int64, onlyPublished bool) ([]model.Chunk, error) {
	rows, err := s.pool.Query(ctx, `select id, doc_id, coalesce(clause_no,''), coalesce(chapter,''),
		seq, coalesce(title,''), content, char_count, part, published
		from doc_chunk where doc_id=$1 and (not $2 or published) order by seq`, docID, onlyPublished)
	if err != nil {
		return nil, wrapErr("查询切片", err)
	}
	defer rows.Close()
	var out []model.Chunk
	for rows.Next() {
		var c model.Chunk
		if err := rows.Scan(&c.ID, &c.DocID, &c.ClauseNo, &c.Chapter, &c.Seq, &c.Title,
			&c.Content, &c.CharCount, &c.Part, &c.Published); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetChunk 取单个切片（供原文定位）。
func (s *Store) GetChunk(ctx context.Context, id int64) (*model.Chunk, error) {
	var c model.Chunk
	err := s.pool.QueryRow(ctx, `select c.id, c.doc_id, coalesce(c.clause_no,''), coalesce(c.chapter,''),
		c.seq, coalesce(c.title,''), c.content, c.char_count, c.part, c.published, d.title
		from doc_chunk c join doc d on d.id=c.doc_id where c.id=$1`, id).
		Scan(&c.ID, &c.DocID, &c.ClauseNo, &c.Chapter, &c.Seq, &c.Title, &c.Content,
			&c.CharCount, &c.Part, &c.Published, &c.DocTitle)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询切片", err)
	}
	return &c, nil
}

// PublishDoc 发布文档：切片置为已发布，文档状态改为 published。
// 只有审核通过（pending_review）的文档可以发布。
func (s *Store) PublishDoc(ctx context.Context, docID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `update doc set status='published', published_at=now(), updated_at=now(),
		version=version+1 where id=$1 and status='pending_review'`, docID)
	if err != nil {
		return wrapErr("发布文档", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	if _, err := tx.Exec(ctx, `update doc_chunk set published=true where doc_id=$1`, docID); err != nil {
		return wrapErr("发布切片", err)
	}
	return tx.Commit(ctx)
}

// ListPublishedChunks 返回全部已发布切片，供向量装载与检索。
func (s *Store) ListPublishedChunks(ctx context.Context) ([]model.Chunk, error) {
	rows, err := s.pool.Query(ctx, `select c.id, c.doc_id, coalesce(c.clause_no,''), coalesce(c.chapter,''),
		c.seq, coalesce(c.title,''), c.content, c.char_count, c.part, c.published, d.title
		from doc_chunk c join doc d on d.id = c.doc_id
		where c.published and d.status='published' order by c.id`)
	if err != nil {
		return nil, wrapErr("查询已发布切片", err)
	}
	defer rows.Close()
	var out []model.Chunk
	for rows.Next() {
		var c model.Chunk
		if err := rows.Scan(&c.ID, &c.DocID, &c.ClauseNo, &c.Chapter, &c.Seq, &c.Title,
			&c.Content, &c.CharCount, &c.Part, &c.Published, &c.DocTitle); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveEmbeddings 批量写入向量。
func (s *Store) SaveEmbeddings(ctx context.Context, modelName string, ids []int64, vecs [][]float32) error {
	if len(ids) != len(vecs) {
		return fmt.Errorf("向量数量与切片数量不一致")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for i, id := range ids {
		if _, err := tx.Exec(ctx, `insert into doc_embedding(chunk_id, model, dim, vec)
			values($1,$2,$3,$4)
			on conflict (chunk_id) do update set model=excluded.model, dim=excluded.dim,
				vec=excluded.vec, updated_at=now()`, id, modelName, len(vecs[i]), vecs[i]); err != nil {
			return wrapErr("写入向量", err)
		}
	}
	return tx.Commit(ctx)
}

// DeletedEmbeddings 统计并删除文档向量。
func (s *Store) DeletedEmbeddings(ctx context.Context, docID int64) (int64, error) {
	tag, err := s.pool.Exec(ctx, `delete from doc_embedding e using doc_chunk c
		where c.id = e.chunk_id and c.doc_id = $1`, docID)
	if err != nil {
		return 0, wrapErr("删除向量", err)
	}
	return tag.RowsAffected(), nil
}

// EmbeddedChunk 是装载检索索引所需的向量与元数据。
type EmbeddedChunk struct {
	ChunkID  int64
	DocID    int64
	DocTitle string
	ClauseNo string
	Chapter  string
	Content  string
	Seq      int
	Vec      []float32
}

// LoadEmbeddings 装载全部已发布切片及其向量。
func (s *Store) LoadEmbeddings(ctx context.Context) ([]EmbeddedChunk, error) {
	rows, err := s.pool.Query(ctx, `select c.id, c.doc_id, d.title, coalesce(c.clause_no,''),
		coalesce(c.chapter,''), c.content, c.seq, e.vec
		from doc_embedding e
		join doc_chunk c on c.id = e.chunk_id
		join doc d on d.id = c.doc_id
		where c.published and d.status='published'
		order by c.id`)
	if err != nil {
		return nil, wrapErr("装载向量", err)
	}
	defer rows.Close()
	var out []EmbeddedChunk
	for rows.Next() {
		var e EmbeddedChunk
		if err := rows.Scan(&e.ChunkID, &e.DocID, &e.DocTitle, &e.ClauseNo, &e.Chapter,
			&e.Content, &e.Seq, &e.Vec); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
