package store

import (
	"context"
	"encoding/json"
)

// QuizExportRow 题库导出/导入行。
type QuizExportRow struct {
	Status   string
	Job      string
	Category string
	Stem     string
	Options  []string
	Answer   string
	Analysis string
}

// QuizQuestionsForExport 导出全部题目（含岗位/分类/状态）。
func (s *Store) QuizQuestionsForExport(ctx context.Context) ([]QuizExportRow, error) {
	rows, err := s.pool.Query(ctx, `select coalesce(q.status,'published'),
		coalesce(nullif(q.job,''), nullif(li.post,''), '通用'),
		coalesce(nullif(q.category,''), li.category, ''),
		q.stem, q.options, q.answer, coalesce(q.analysis,'')
		from quiz_question q left join learn_item li on li.id=q.item_id
		order by q.id`)
	if err != nil {
		return nil, wrapErr("导出题库", err)
	}
	defer rows.Close()
	out := []QuizExportRow{}
	for rows.Next() {
		var r QuizExportRow
		var opts []byte
		if err := rows.Scan(&r.Status, &r.Job, &r.Category, &r.Stem, &opts, &r.Answer, &r.Analysis); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(opts, &r.Options)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ImportQuizQuestion 导入题目（默认待审核）。
func (s *Store) ImportQuizQuestion(ctx context.Context, stem string, options []string, answer, analysis, job, category string) (int64, error) {
	opts, _ := json.Marshal(options)
	var id int64
	err := s.pool.QueryRow(ctx, `insert into quiz_question(stem, options, answer, analysis, difficulty, status, job, category)
		values ($1,$2,$3,nullif($4,''),'normal','pending',$5,$6) returning id`,
		stem, opts, answer, analysis, job, category).Scan(&id)
	return id, wrapErr("导入题目", err)
}
