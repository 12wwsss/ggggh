package store

import (
	"context"
	"encoding/json"

	"coalrag/internal/model"
)

// RiskPoint 岗位风险点。
type RiskPoint struct {
	ID          int64    `json:"id"`
	Post        string   `json:"post"`
	Category    string   `json:"category"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Measures    []string `json:"measures"`
	Level       string   `json:"level"`
	Clauses     []string `json:"clause_refs"`
}

// ListRiskPoints 按岗位取风险点（job 为空取全部）。
func (s *Store) ListRiskPoints(ctx context.Context, job string, limit int) ([]RiskPoint, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx, `select id, post, category, title, description, measures, level, clause_refs
		from risk_point where status='published' and ($1='' or post=$1 or post='通用')
		order by sort, id limit $2`, job, limit)
	if err != nil {
		return nil, wrapErr("查询风险点", err)
	}
	defer rows.Close()
	out := []RiskPoint{}
	for rows.Next() {
		var r RiskPoint
		var measures, clauses []byte
		if err := rows.Scan(&r.ID, &r.Post, &r.Category, &r.Title, &r.Description, &measures, &r.Level, &clauses); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(measures, &r.Measures)
		_ = json.Unmarshal(clauses, &r.Clauses)
		if r.Measures == nil {
			r.Measures = []string{}
		}
		if r.Clauses == nil {
			r.Clauses = []string{}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateRiskPoint 新增风险点。
func (s *Store) CreateRiskPoint(ctx context.Context, r *RiskPoint) (int64, error) {
	measures, _ := json.Marshal(r.Measures)
	clauses, _ := json.Marshal(r.Clauses)
	var id int64
	err := s.pool.QueryRow(ctx, `insert into risk_point(post, category, title, description, measures, level, clause_refs)
		values ($1,$2,$3,$4,$5,$6,$7) returning id`,
		r.Post, r.Category, r.Title, r.Description, measures, r.Level, clauses).Scan(&id)
	return id, wrapErr("写入风险点", err)
}

// DeleteRiskPoint 删除风险点。
func (s *Store) DeleteRiskPoint(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `delete from risk_point where id=$1`, id)
	if err != nil {
		return wrapErr("删除风险点", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

var _ = model.LearnItem{}
