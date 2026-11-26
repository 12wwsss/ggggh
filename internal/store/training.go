package store

import (
	"context"
	"encoding/json"

	"coalrag/internal/model"
)

// TrainingStage 带培阶段。
type TrainingStage struct {
	ID    int64    `json:"id"`
	Sort  int      `json:"sort"`
	Week  string   `json:"week"`
	Goal  string   `json:"goal"`
	Tasks []string `json:"tasks"`
	Done  bool     `json:"done"`
}

// TrainingPlan 带培计划。
type TrainingPlan struct {
	ID           int64           `json:"id"`
	UserID       int64           `json:"user_id"`
	UserDeptID   *int64          `json:"user_dept_id"`
	UserName     string          `json:"user_name"`
	DeptName     string          `json:"dept_name"`
	Title        string          `json:"title"`
	Mentor       string          `json:"mentor"`
	MentorUserID *int64          `json:"mentor_user_id"`
	TotalWeeks   int             `json:"total_weeks"`
	Target       string          `json:"target"`
	Status       string          `json:"status"`
	Stages       []TrainingStage `json:"stages"`
	StageTotal   int             `json:"stage_total"`
	StageDone    int             `json:"stage_done"`
}

const trainingPlanCols = `select p.id, p.user_id, p.title, coalesce(p.mentor,''), p.total_weeks,
	coalesce(p.target,''), p.status, coalesce(u.name,''), coalesce(d.name,''), u.dept_id,
	coalesce(m.name,''), p.mentor_user_id
	from training_plan p
	left join org_user u on u.id=p.user_id
	left join org_user m on m.id=p.mentor_user_id
	left join org_dept d on d.id=u.dept_id`

func scanPlan(row interface{ Scan(...any) error }, p *TrainingPlan) error {
	return row.Scan(&p.ID, &p.UserID, &p.Title, &p.Mentor, &p.TotalWeeks, &p.Target, &p.Status,
		&p.UserName, &p.DeptName, &p.UserDeptID, &p.Mentor, &p.MentorUserID)
}

func (s *Store) loadStages(ctx context.Context, p *TrainingPlan) error {
	rows, err := s.pool.Query(ctx, `select id, sort, coalesce(week,''), goal, tasks, done
		from training_stage where plan_id=$1 order by sort, id`, p.ID)
	if err != nil {
		return wrapErr("查询带培阶段", err)
	}
	defer rows.Close()
	for rows.Next() {
		var st TrainingStage
		var tasks []byte
		if err := rows.Scan(&st.ID, &st.Sort, &st.Week, &st.Goal, &tasks, &st.Done); err != nil {
			return err
		}
		_ = json.Unmarshal(tasks, &st.Tasks)
		if st.Tasks == nil {
			st.Tasks = []string{}
		}
		p.Stages = append(p.Stages, st)
	}
	p.StageTotal = len(p.Stages)
	for _, st := range p.Stages {
		if st.Done {
			p.StageDone++
		}
	}
	return rows.Err()
}

// GetTrainingPlan 取某人当前带培计划。
func (s *Store) GetTrainingPlan(ctx context.Context, userID int64) (*TrainingPlan, error) {
	var p TrainingPlan
	err := scanPlan(s.pool.QueryRow(ctx, trainingPlanCols+` where p.user_id=$1 and p.status<>'paused'
		order by p.id desc limit 1`, userID), &p)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询带培计划", err)
	}
	return &p, s.loadStages(ctx, &p)
}

// GetTrainingPlanByID 按计划 id 取（管理端）。
func (s *Store) GetTrainingPlanByID(ctx context.Context, id int64) (*TrainingPlan, error) {
	var p TrainingPlan
	err := scanPlan(s.pool.QueryRow(ctx, trainingPlanCols+` where p.id=$1`, id), &p)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询带培计划", err)
	}
	return &p, s.loadStages(ctx, &p)
}

// GetPlanIDByStage 通过阶段找计划。
func (s *Store) GetPlanIDByStage(ctx context.Context, stageID int64) (int64, error) {
	var planID int64
	err := s.pool.QueryRow(ctx, `select plan_id from training_stage where id=$1`, stageID).Scan(&planID)
	if isNoRows(err) {
		return 0, ErrNotFound
	}
	return planID, wrapErr("查询带培计划", err)
}

// ListTrainingPlansScoped 按部门/师傅账号过滤。
func (s *Store) ListTrainingPlansScoped(ctx context.Context, deptID, mentorID *int64, limit, offset int) ([]TrainingPlan, int, error) {
	where := ` where (($1::bigint is not null and u.dept_id=$1) or ($2::bigint is not null and p.mentor_user_id=$2))`
	rows, err := s.pool.Query(ctx, trainingPlanCols+where+`
		order by p.id desc limit $3 offset $4`, deptID, mentorID, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询带培计划", err)
	}
	defer rows.Close()
	var out []TrainingPlan
	for rows.Next() {
		var p TrainingPlan
		if err := scanPlan(rows, &p); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	var total int
	err = s.pool.QueryRow(ctx, `select count(*) from training_plan p
		left join org_user u on u.id=p.user_id`+where, deptID, mentorID).Scan(&total)
	return out, total, wrapErr("统计带培计划", err)
}

// ListTrainingPlans 管理端：全部计划。
func (s *Store) ListTrainingPlans(ctx context.Context, limit, offset int) ([]TrainingPlan, int, error) {
	rows, err := s.pool.Query(ctx, trainingPlanCols+` order by p.id desc limit $1 offset $2`, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询带培计划", err)
	}
	defer rows.Close()
	var out []TrainingPlan
	for rows.Next() {
		var p TrainingPlan
		if err := scanPlan(rows, &p); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	var total int
	err = s.pool.QueryRow(ctx, `select count(*) from training_plan`).Scan(&total)
	return out, total, wrapErr("统计带培计划", err)
}

// CreateTrainingPlan 新建计划并生成 4 周默认阶段。
func (s *Store) CreateTrainingPlan(ctx context.Context, userID, mentorUserID int64, title string, totalWeeks int, target string) (int64, error) {
	var mentorName string
	if err := s.pool.QueryRow(ctx, `select name from org_user where id=$1`, mentorUserID).Scan(&mentorName); err != nil {
		return 0, wrapErr("查询师傅", err)
	}
	if totalWeeks <= 0 {
		totalWeeks = 4
	}
	var id int64
	err := s.pool.QueryRow(ctx, `insert into training_plan(user_id, title, mentor, mentor_user_id, total_weeks, target)
		values ($1,$2,$3,$4,$5,$6) returning id`,
		userID, title, mentorName, mentorUserID, totalWeeks, target).Scan(&id)
	if err != nil {
		return 0, wrapErr("创建带培计划", err)
	}
	stages := []struct {
		week, goal string
		tasks      []string
	}{
		{"第 1 周", "入矿教育与安全规程", []string{"完成三级安全教育", "学习《煤矿安全规程》总则与入井规定"}},
		{"第 2 周", "岗位跟班学习", []string{"跟随师傅下井熟悉作业环境", "认识本岗位设备与安全设施"}},
		{"第 3 周", "实操训练", []string{"本岗位关键操作实操", "应急处置演练"}},
		{"第 4 周", "考核与独立上岗", []string{"通过岗位安全考试", "师傅评价合格后独立上岗"}},
	}
	for i, st := range stages {
		tasks, _ := json.Marshal(st.tasks)
		if _, err := s.pool.Exec(ctx, `insert into training_stage(plan_id, sort, week, goal, tasks)
			values ($1,$2,$3,$4,$5)`, id, i+1, st.week, st.goal, tasks); err != nil {
			return 0, wrapErr("创建带培阶段", err)
		}
	}
	return id, nil
}

// UpdatePlanMentor 更换带培师傅。
func (s *Store) UpdatePlanMentor(ctx context.Context, planID, mentorUserID int64) error {
	var name string
	if err := s.pool.QueryRow(ctx, `select name from org_user where id=$1 and status='active'`, mentorUserID).Scan(&name); err != nil {
		return wrapErr("查询师傅", err)
	}
	tag, err := s.pool.Exec(ctx, `update training_plan set mentor=$2, mentor_user_id=$3 where id=$1`,
		planID, name, mentorUserID)
	if err != nil {
		return wrapErr("更换师傅", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetTrainingStageDone 标记阶段完成/未完成。
func (s *Store) SetTrainingStageDone(ctx context.Context, planID, stageID int64, done bool) error {
	_, err := s.pool.Exec(ctx, `update training_stage set done=$3,
		done_at = case when $3 then now() else null end
		where id=$1 and plan_id=$2`, stageID, planID, done)
	if err != nil {
		return wrapErr("更新带培阶段", err)
	}
	return s.refreshPlanStatus(ctx, planID)
}

// refreshPlanStatus 阶段全部完成时把计划置为 done，取消完成时恢复 active。
func (s *Store) refreshPlanStatus(ctx context.Context, planID int64) error {
	var total, done int
	if err := s.pool.QueryRow(ctx, `select count(*), count(*) filter (where done)
		from training_stage where plan_id=$1`, planID).Scan(&total, &done); err != nil {
		return wrapErr("统计带培阶段", err)
	}
	status := "active"
	if total > 0 && done == total {
		status = "done"
	}
	_, err := s.pool.Exec(ctx, `update training_plan set status=$2 where id=$1`, planID, status)
	return wrapErr("更新带培状态", err)
}

// SetStageDoneByID 管理端标记阶段。
func (s *Store) SetStageDoneByID(ctx context.Context, stageID int64, done bool) error {
	var planID int64
	if err := s.pool.QueryRow(ctx, `select plan_id from training_stage where id=$1`, stageID).Scan(&planID); err != nil {
		return wrapErr("查询带培阶段", err)
	}
	tag, err := s.pool.Exec(ctx, `update training_stage set done=$2,
		done_at = case when $2 then now() else null end where id=$1`, stageID, done)
	if err != nil {
		return wrapErr("更新带培阶段", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return s.refreshPlanStatus(ctx, planID)
}

var _ = model.LearnRecord{}
