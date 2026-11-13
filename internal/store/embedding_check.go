package store

import "context"

// CountEmbeddingsWithDim 统计维度不等于 dim 的向量条数，用于启动校验。
func (s *Store) CountEmbeddingsWithDim(ctx context.Context, dim int) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from doc_embedding where dim <> $1`, dim).Scan(&n)
	return n, wrapErr("校验向量维度", err)
}
