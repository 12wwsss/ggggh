package store

import (
	"context"
	"encoding/json"

	"coalrag/internal/model"
)

// WrongItem 错题本条目（题目内容 + 错误次数）。
type WrongItem struct {
	CompatQuestion
	Times int `json:"times"`
}

// SetWrongQuestion 答错收录/答对移出。
func (s *Store) SetWrongQuestion(ctx context.Context, userID, questionID int64, ok bool) error {
	if ok {
		_, err := s.pool.Exec(ctx, `delete from wrong_book where user_id=$1 and question_id=$2`, userID, questionID)
		return wrapErr("移出错题", err)
	}
	_, err := s.pool.Exec(ctx, `insert into wrong_book(user_id, question_id) values ($1,$2)
		on conflict (user_id, question_id) do update set times=wrong_book.times+1, updated_at=now()`,
		userID, questionID)
	return wrapErr("收录错题", err)
}

// ListWrongQuestions 错题本列表。
func (s *Store) ListWrongQuestions(ctx context.Context, userID int64) ([]WrongItem, error) {
	rows, err := s.pool.Query(ctx, `select q.id, q.stem, q.options, q.answer, coalesce(q.analysis,''),
		coalesce(li.post,''), coalesce(li.category,''), wb.times
		from wrong_book wb join quiz_question q on q.id=wb.question_id
		left join learn_item li on li.id=q.item_id
		where wb.user_id=$1 order by wb.updated_at desc`, userID)
	if err != nil {
		return nil, wrapErr("查询错题本", err)
	}
	defer rows.Close()
	out := []WrongItem{}
	for rows.Next() {
		var w WrongItem
		var opts []byte
		if err := rows.Scan(&w.ID, &w.Stem, &opts, &w.Answer, &w.Analysis, &w.Job, &w.Category, &w.Times); err != nil {
			return nil, err
		}
		var list []string
		_ = json.Unmarshal(opts, &list)
		w.Options = StripOptionPrefix(list)
		out = append(out, w)
	}
	return out, rows.Err()
}

// WrongCount 错题数量（首页/我的统计）。
func (s *Store) WrongCount(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from wrong_book where user_id=$1`, userID).Scan(&n)
	return n, wrapErr("统计错题", err)
}

var _ = model.QuizQuestion{}
