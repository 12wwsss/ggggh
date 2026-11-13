package store

import (
	"context"
	"encoding/json"

	"coalrag/internal/model"
)

// ---------- 学习条目 ----------

// ListLearnItems 分页查询学习条目。
func (s *Store) ListLearnItems(ctx context.Context, status string, deptID *int64, limit, offset int) ([]model.LearnItem, int, error) {
	rows, err := s.pool.Query(ctx, `select id, title, category, coalesce(post,''), dept_id, content,
		clause_refs, doc_id, sort, status, created_at, count(*) over() as total
		from learn_item
		where ($1='' or status=$1) and ($2::bigint is null or dept_id is null or dept_id=$2)
		order by sort, id limit $3 offset $4`, status, deptID, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询学习条目", err)
	}
	defer rows.Close()
	var out []model.LearnItem
	total := 0
	for rows.Next() {
		var it model.LearnItem
		var refs []byte
		if err := rows.Scan(&it.ID, &it.Title, &it.Category, &it.Post, &it.DeptID, &it.Content,
			&refs, &it.DocID, &it.Sort, &it.Status, &it.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(refs, &it.ClauseRefs)
		if it.ClauseRefs == nil {
			it.ClauseRefs = []string{}
		}
		out = append(out, it)
	}
	return out, total, rows.Err()
}

// GetLearnItem 取学习条目。
func (s *Store) GetLearnItem(ctx context.Context, id int64) (*model.LearnItem, error) {
	var it model.LearnItem
	var refs []byte
	err := s.pool.QueryRow(ctx, `select id, title, category, coalesce(post,''), dept_id, content,
		clause_refs, doc_id, sort, status, created_at from learn_item where id=$1`, id).
		Scan(&it.ID, &it.Title, &it.Category, &it.Post, &it.DeptID, &it.Content, &refs,
			&it.DocID, &it.Sort, &it.Status, &it.CreatedAt)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询学习条目", err)
	}
	_ = json.Unmarshal(refs, &it.ClauseRefs)
	if it.ClauseRefs == nil {
		it.ClauseRefs = []string{}
	}
	return &it, nil
}

// CreateLearnItem 新建学习条目。
func (s *Store) CreateLearnItem(ctx context.Context, it *model.LearnItem) (int64, error) {
	refs, _ := json.Marshal(arrStr(it.ClauseRefs))
	var id int64
	err := s.pool.QueryRow(ctx, `insert into learn_item
		(title, category, post, dept_id, content, clause_refs, doc_id, sort, status)
		values ($1,$2,nullif($3,''),$4,$5,$6,$7,$8,$9) returning id`,
		it.Title, it.Category, it.Post, it.DeptID, it.Content, refs, it.DocID, it.Sort, it.Status).Scan(&id)
	return id, wrapErr("新建学习条目", err)
}

// UpdateLearnItem 更新学习条目。
func (s *Store) UpdateLearnItem(ctx context.Context, it *model.LearnItem) error {
	refs, _ := json.Marshal(it.ClauseRefs)
	tag, err := s.pool.Exec(ctx, `update learn_item set title=$2, category=$3, post=nullif($4,''),
		dept_id=$5, content=$6, clause_refs=$7, doc_id=$8, sort=$9, status=$10, updated_at=now()
		where id=$1`, it.ID, it.Title, it.Category, it.Post, it.DeptID, it.Content, refs,
		it.DocID, it.Sort, it.Status)
	if err != nil {
		return wrapErr("更新学习条目", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------- 学习记录 ----------

// GetLearnRecord 取某人对某条目的学习记录。
func (s *Store) GetLearnRecord(ctx context.Context, itemID, userID int64) (*model.LearnRecord, error) {
	var r model.LearnRecord
	err := s.pool.QueryRow(ctx, `select id, item_id, user_id, status, progress, duration_sec,
		started_at, completed_at from learn_record where item_id=$1 and user_id=$2`, itemID, userID).
		Scan(&r.ID, &r.ItemID, &r.UserID, &r.Status, &r.Progress, &r.DurationSec, &r.StartedAt, &r.CompletedAt)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询学习记录", err)
	}
	return &r, nil
}

// SaveLearnProgress 上报学习进度；进度只增不减，完成后不再回退。
func (s *Store) SaveLearnProgress(ctx context.Context, itemID, userID int64, progress, durationSec int) (*model.LearnRecord, error) {
	status := "in_progress"
	if progress >= 100 {
		status = "completed"
		progress = 100
	}
	var r model.LearnRecord
	err := s.pool.QueryRow(ctx, `insert into learn_record
		(item_id, user_id, status, progress, duration_sec, started_at, completed_at, updated_at)
		values ($1,$2,$3,$4,$5,now(), case when $3='completed' then now() end, now())
		on conflict (item_id, user_id) do update set
			progress = greatest(learn_record.progress, excluded.progress),
			duration_sec = learn_record.duration_sec + excluded.duration_sec,
			status = case when learn_record.status='completed' then 'completed' else excluded.status end,
			started_at = coalesce(learn_record.started_at, now()),
			completed_at = case when learn_record.status='completed' then learn_record.completed_at
			                    when excluded.status='completed' then now() else learn_record.completed_at end,
			updated_at = now()
		returning id, item_id, user_id, status, progress, duration_sec, started_at, completed_at`,
		itemID, userID, status, progress, durationSec).
		Scan(&r.ID, &r.ItemID, &r.UserID, &r.Status, &r.Progress, &r.DurationSec, &r.StartedAt, &r.CompletedAt)
	return &r, wrapErr("保存学习进度", err)
}

// ListLearnRecords 按范围查询学习记录。
func (s *Store) ListLearnRecords(ctx context.Context, deptID, userID *int64, itemID *int64, limit, offset int) ([]model.LearnRecord, int, error) {
	rows, err := s.pool.Query(ctx, `select r.id, r.item_id, r.user_id, u.name, r.status, r.progress,
		r.duration_sec, r.started_at, r.completed_at, count(*) over() as total
		from learn_record r
		join org_user u on u.id=r.user_id
		where ($1::bigint is null or u.dept_id=$1)
		  and ($2::bigint is null or r.user_id=$2)
		  and ($3::bigint is null or r.item_id=$3)
		order by r.id desc limit $4 offset $5`, deptID, userID, itemID, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询学习记录", err)
	}
	defer rows.Close()
	var out []model.LearnRecord
	total := 0
	for rows.Next() {
		var r model.LearnRecord
		if err := rows.Scan(&r.ID, &r.ItemID, &r.UserID, &r.UserName, &r.Status, &r.Progress,
			&r.DurationSec, &r.StartedAt, &r.CompletedAt, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// ---------- 小测 ----------

// ListQuizQuestions 查询题目；itemID 为空时查全部。
func (s *Store) ListQuizQuestions(ctx context.Context, itemID *int64, ids []int64) ([]model.QuizQuestion, error) {
	q := `select id, item_id, stem, options, answer, coalesce(analysis,''), difficulty
		from quiz_question
		where ($1::bigint is null or item_id=$1)
		  and (cardinality($2::bigint[]) = 0 or id = any($2))
		order by id`
	rows, err := s.pool.Query(ctx, q, itemID, arr64(ids))
	if err != nil {
		return nil, wrapErr("查询题目", err)
	}
	defer rows.Close()
	var out []model.QuizQuestion
	for rows.Next() {
		var qq model.QuizQuestion
		var opts []byte
		if err := rows.Scan(&qq.ID, &qq.ItemID, &qq.Stem, &opts, &qq.Answer, &qq.Analysis, &qq.Difficulty); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(opts, &qq.Options)
		out = append(out, qq)
	}
	return out, rows.Err()
}

// CreateQuizQuestion 新建题目。
func (s *Store) CreateQuizQuestion(ctx context.Context, q *model.QuizQuestion) (int64, error) {
	opts, _ := json.Marshal(q.Options)
	var id int64
	err := s.pool.QueryRow(ctx, `insert into quiz_question(item_id, stem, options, answer, analysis, difficulty)
		values ($1,$2,$3,$4,nullif($5,''),$6) returning id`,
		q.ItemID, q.Stem, opts, q.Answer, q.Analysis, q.Difficulty).Scan(&id)
	return id, wrapErr("新建题目", err)
}
