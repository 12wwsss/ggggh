package store

import (
	"context"

	"coalrag/internal/model"
)

// GetIssueAttachment 取单个附件记录（含落盘路径）。
func (s *Store) GetIssueAttachment(ctx context.Context, id int64) (*model.IssueAttachment, error) {
	var a model.IssueAttachment
	err := s.pool.QueryRow(ctx, `select id, issue_id, file_name, file_size, coalesce(sha256,''),
		uploader_id, created_at from issue_attachment where id=$1`, id).
		Scan(&a.ID, &a.IssueID, &a.FileName, &a.FileSize, &a.SHA256, &a.UploaderID, &a.CreatedAt)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询附件", err)
	}
	return &a, nil
}
