package store

import (
	"context"

	"coalrag/internal/model"
)

// FindDocBySHA 按内容哈希查最近一次入库的文档（解析失败的不算，允许重新上传）。
func (s *Store) FindDocBySHA(ctx context.Context, sha string) (*model.Doc, error) {
	if sha == "" {
		return nil, ErrNotFound
	}
	row := s.pool.QueryRow(ctx, `select `+sqlDocCols+sqlDocFrom+
		` where d.sha256=$1 and d.status <> 'failed' order by d.id desc limit 1`, sha)
	var d model.Doc
	err := row.Scan(&d.ID, &d.Title, &d.Category, &d.SourceName, &d.FileSize, &d.SHA256,
		&d.Status, &d.Version, &d.ChunkCount, &d.UploaderID, &d.Uploader, &d.ErrorMsg,
		&d.CreatedAt, &d.UpdatedAt, &d.PublishedAt)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("按内容查重", err)
	}
	return &d, nil
}
