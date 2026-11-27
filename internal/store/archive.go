package store

import (
	"context"

	"coalrag/internal/model"
)

// ArchiveLearn 一条学习记录（含未开始条目）。
type ArchiveLearn struct {
	Title    string
	Category string
	Status   string
	Progress int
	Minutes  int
	Updated  string
}

// ArchiveExam 一次考试。
type ArchiveExam struct {
	Paper  string
	Score  int
	Passed bool
	Status string
	At     string
}

// ArchiveBroadcast 一条宣贯签收。
type ArchiveBroadcast struct {
	Title  string
	Status string
	At     string
}

// ArchiveWrong 一道错题。
type ArchiveWrong struct {
	Stem  string
	Times int
	At    string
}

// ArchiveIssue 一条上报的问题。
type ArchiveIssue struct {
	Code   string
	Title  string
	Status string
	Post   string
	At     string
	Closed string
}

// ArchiveTraining 一份带培计划及其小结。
type ArchiveTraining struct {
	Title    string
	Mentor   string
	Status   string
	Weeks    int
	Stages   string
	Notes    string
	CreatedA string
}

// UserArchive 是「一人一档」需要的全部数据。
type UserArchive struct {
	User       model.User
	Learn      []ArchiveLearn
	Exams      []ArchiveExam
	Broadcasts []ArchiveBroadcast
	Wrongs     []ArchiveWrong
	Issues     []ArchiveIssue
	Training   []ArchiveTraining
}

// UserArchive 汇总某人的学习、考试、宣贯、错题、问题与带培记录。
func (s *Store) UserArchive(ctx context.Context, userID int64) (*UserArchive, error) {
	u, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	a := &UserArchive{
		User:       *u,
		Learn:      []ArchiveLearn{},
		Exams:      []ArchiveExam{},
		Broadcasts: []ArchiveBroadcast{},
		Wrongs:     []ArchiveWrong{},
		Issues:     []ArchiveIssue{},
		Training:   []ArchiveTraining{},
	}

	if rows, err := s.pool.Query(ctx, `select li.title, li.category,
			coalesce(r.status,'not_started'), coalesce(r.progress,0), coalesce(r.duration_sec,0),
			coalesce(to_char(r.updated_at,'YYYY-MM-DD HH24:MI'),'')
		from learn_item li
		left join learn_record r on r.item_id=li.id and r.user_id=$1
		where li.status='published'
		order by li.sort, li.id`, userID); err != nil {
		return nil, wrapErr("统计学习档案", err)
	} else {
		for rows.Next() {
			var r ArchiveLearn
			if err := rows.Scan(&r.Title, &r.Category, &r.Status, &r.Progress, &r.Minutes, &r.Updated); err != nil {
				rows.Close()
				return nil, err
			}
			r.Minutes = r.Minutes / 60
			a.Learn = append(a.Learn, r)
		}
		rows.Close()
	}

	if rows, err := s.pool.Query(ctx, `select p.title, r.score, r.passed, r.status,
			coalesce(to_char(r.submitted_at,'YYYY-MM-DD HH24:MI'),'')
		from exam_record r join exam_paper p on p.id=r.paper_id
		where r.user_id=$1 order by r.id desc`, userID); err != nil {
		return nil, wrapErr("统计考试记录", err)
	} else {
		for rows.Next() {
			var r ArchiveExam
			if err := rows.Scan(&r.Paper, &r.Score, &r.Passed, &r.Status, &r.At); err != nil {
				rows.Close()
				return nil, err
			}
			a.Exams = append(a.Exams, r)
		}
		rows.Close()
	}

	if rows, err := s.pool.Query(ctx, `select t.title, r.status,
			coalesce(to_char(coalesce(r.ack_at, r.read_at),'YYYY-MM-DD HH24:MI'),'')
		from broadcast_record r join broadcast_task t on t.id=r.task_id
		where r.user_id=$1 order by r.id desc`, userID); err != nil {
		return nil, wrapErr("统计宣贯记录", err)
	} else {
		for rows.Next() {
			var r ArchiveBroadcast
			if err := rows.Scan(&r.Title, &r.Status, &r.At); err != nil {
				rows.Close()
				return nil, err
			}
			a.Broadcasts = append(a.Broadcasts, r)
		}
		rows.Close()
	}

	if rows, err := s.pool.Query(ctx, `select q.stem, wb.times,
			coalesce(to_char(wb.updated_at,'YYYY-MM-DD HH24:MI'),'')
		from wrong_book wb join quiz_question q on q.id=wb.question_id
		where wb.user_id=$1 order by wb.updated_at desc`, userID); err != nil {
		return nil, wrapErr("统计错题", err)
	} else {
		for rows.Next() {
			var r ArchiveWrong
			if err := rows.Scan(&r.Stem, &r.Times, &r.At); err != nil {
				rows.Close()
				return nil, err
			}
			a.Wrongs = append(a.Wrongs, r)
		}
		rows.Close()
	}

	if rows, err := s.pool.Query(ctx, `select code, title, status, coalesce(post,''),
			to_char(created_at,'YYYY-MM-DD HH24:MI'),
			coalesce(to_char(closed_at,'YYYY-MM-DD HH24:MI'),'')
		from issue where reporter_id=$1 order by id desc limit 500`, userID); err != nil {
		return nil, wrapErr("统计问题记录", err)
	} else {
		for rows.Next() {
			var r ArchiveIssue
			if err := rows.Scan(&r.Code, &r.Title, &r.Status, &r.Post, &r.At, &r.Closed); err != nil {
				rows.Close()
				return nil, err
			}
			a.Issues = append(a.Issues, r)
		}
		rows.Close()
	}

	if rows, err := s.pool.Query(ctx, `select p.title, coalesce(m.name,''), p.status, p.total_weeks,
			to_char(p.created_at,'YYYY-MM-DD'),
			(select count(*) from training_stage st where st.plan_id=p.id and st.done) || '/' ||
			(select count(*) from training_stage st where st.plan_id=p.id),
			coalesce((select string_agg(n.period||' '||coalesce(u2.name,'')||'：'||n.content, E'\n' order by n.id)
			         from training_note n left join org_user u2 on u2.id=n.author_id
			         where n.plan_id=p.id), '')
		from training_plan p
		left join org_user m on m.id=p.mentor_user_id
		where p.user_id=$1 order by p.id desc`, userID); err != nil {
		return nil, wrapErr("统计带培记录", err)
	} else {
		for rows.Next() {
			var r ArchiveTraining
			if err := rows.Scan(&r.Title, &r.Mentor, &r.Status, &r.Weeks, &r.CreatedA, &r.Stages, &r.Notes); err != nil {
				rows.Close()
				return nil, err
			}
			a.Training = append(a.Training, r)
		}
		rows.Close()
	}
	return a, nil
}
