package store

import (
	"context"
	"time"
)

// TrainingNote 是带培师傅写的月度小结/期满总结。
type TrainingNote struct {
	ID        int64     `json:"id"`
	PlanID    int64     `json:"plan_id"`
	AuthorID  *int64    `json:"author_id,omitempty"`
	Author    string    `json:"author"`
	Kind      string    `json:"kind"`
	Period    string    `json:"period"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// AddTrainingNote 写一条带培小结。
func (s *Store) AddTrainingNote(ctx context.Context, n *TrainingNote) (int64, error) {
	if n.Kind == "" {
		n.Kind = "monthly"
	}
	var id int64
	err := s.pool.QueryRow(ctx, `insert into training_note(plan_id, author_id, kind, period, content)
		values ($1,$2,$3,$4,$5) returning id`,
		n.PlanID, n.AuthorID, n.Kind, n.Period, n.Content).Scan(&id)
	return id, wrapErr("写入带培小结", err)
}

// ListTrainingNotes 某份计划的全部小结（新的在前）。
func (s *Store) ListTrainingNotes(ctx context.Context, planID int64) ([]TrainingNote, error) {
	rows, err := s.pool.Query(ctx, `select n.id, n.plan_id, n.author_id, coalesce(u.name,''),
		n.kind, n.period, n.content, n.created_at
		from training_note n left join org_user u on u.id=n.author_id
		where n.plan_id=$1 order by n.id desc`, planID)
	if err != nil {
		return nil, wrapErr("查询带培小结", err)
	}
	defer rows.Close()
	out := []TrainingNote{}
	for rows.Next() {
		var n TrainingNote
		if err := rows.Scan(&n.ID, &n.PlanID, &n.AuthorID, &n.Author, &n.Kind, &n.Period,
			&n.Content, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
