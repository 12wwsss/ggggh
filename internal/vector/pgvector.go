package vector

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PgVector 是基于 pgvector 扩展的实现。
//
// 依赖：create extension vector。向量以 real[] 落库，检索时拼成
// pgvector 的文本字面量后做 <=> 距离运算，避免同时维护两份向量列。
// 开发环境未安装扩展，此实现按配置启用，未在本地验证。
type PgVector struct {
	pool *pgxpool.Pool
	dim  int
}

// NewPgVector 构造 pgvector 实现。
func NewPgVector(pool *pgxpool.Pool, dim int) *PgVector {
	return &PgVector{pool: pool, dim: dim}
}

// Kind 返回实现名。
func (p *PgVector) Kind() string { return "pgvector" }

func literal(v []float32) string {
	var b strings.Builder
	b.Grow(len(v) * 8)
	b.WriteByte('[')
	for i, x := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(x), 'f', 6, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// Upsert 写入向量。
func (p *PgVector) Upsert(ctx context.Context, docs []Doc) error {
	for _, d := range docs {
		if len(d.Vec) != p.dim {
			return fmt.Errorf("向量维度 %d 与配置 %d 不一致", len(d.Vec), p.dim)
		}
		if _, err := p.pool.Exec(ctx, `insert into doc_embedding(chunk_id, model, dim, vec)
			values($1,$2,$3,$4)
			on conflict (chunk_id) do update set model=excluded.model, dim=excluded.dim,
				vec=excluded.vec, updated_at=now()`,
			d.ChunkID, "pgvector", p.dim, d.Vec); err != nil {
			return err
		}
	}
	return nil
}

// DeleteByDoc 删除某文档向量。
func (p *PgVector) DeleteByDoc(ctx context.Context, docID int64) error {
	_, err := p.pool.Exec(ctx, `delete from doc_embedding e using doc_chunk c
		where c.id = e.chunk_id and c.doc_id = $1`, docID)
	return err
}

// Search 使用 pgvector 的余弦距离运算符。
func (p *PgVector) Search(ctx context.Context, query []float32, topK int) ([]Hit, error) {
	if len(query) != p.dim {
		return nil, fmt.Errorf("查询向量维度 %d 与配置 %d 不一致", len(query), p.dim)
	}
	q := literal(query)
	rows, err := p.pool.Query(ctx, `select c.id, c.doc_id, d.title, coalesce(c.clause_no,''),
		coalesce(c.chapter,''), c.content, c.seq,
		1 - (('[' || array_to_string(e.vec, ',') || ']')::vector <=> $1::vector) as score
		from doc_embedding e
		join doc_chunk c on c.id = e.chunk_id
		join doc d on d.id = c.doc_id
		where c.published and d.status='published'
		order by score desc, c.id limit $2`, q, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hits []Hit
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.ChunkID, &h.DocID, &h.DocTitle, &h.ClauseNo, &h.Chapter,
			&h.Content, &h.Seq, &h.Score); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// Reload 对 pgvector 实现是空操作，检索直接查库。
func (p *PgVector) Reload(context.Context) error { return nil }

// Size 返回向量总条数。
func (p *PgVector) Size() int {
	var n int
	_ = p.pool.QueryRow(context.Background(), `select count(*) from doc_embedding`).Scan(&n)
	return n
}
