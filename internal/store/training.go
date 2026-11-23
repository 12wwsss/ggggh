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
	ID         int64           `json:"id"`
	UserID     int64           `json:"user_id"`
	UserDeptID *int64          `json:"user_dept_id"`
	UserName   string          `json:"user_name"`
	DeptName   string          `json:"dept_name"`
	Title      string          `json:"title"`
	Mentor     string          `json:"mentor"`
	TotalWeeks int             `json:"total_weeks"`
	Target     string          `json:"target"`
	Status     string          `json:"status"`
	Stages     []TrainingStage `json:"stages"`
	StageTotal int             `json:"stage_total"`
	StageDone  int             `json:"stage_done"`
}

// GetTrainingPlan 取某人当前带培计划。
func (s *Store) GetTrainingPlan(ctx context.Context, userID int64) (*TrainingPlan, error) {
	var p TrainingPlan
	err := s.pool.QueryRow(ctx, `select id, user_id, title, coalesce(mentor,''), total_weeks,
		coalesce(target,''), status from training_plan where user_id=$1 and status<>'paused'
		order by id desc limit 1`, userID).
		Scan(&p.ID, &p.UserID, &p.Title, &p.Mentor, &p.TotalWeeks, &p.Target, &p.Status)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询带培计划", err)
	}
	rows, err := s.pool.Query(ctx, `select id, sort, coalesce(week,''), goal, tasks, done
		from training_stage where plan_id=$1 order by sort, id`, p.ID)
	if err != nil {
		return nil, wrapErr("查询带培阶段", err)
	}
	defer rows.Close()
	for rows.Next() {
		var st TrainingStage
		var tasks []byte
		if err := rows.Scan(&st.ID, &st.Sort, &st.Week, &st.Goal, &tasks, &st.Done); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(tasks, &st.Tasks)
		if st.Tasks == nil {
			st.Tasks = []string{}
		}
		p.Stages = append(p.Stages, st)
	}
	return &p, rows.Err()
}

// SetTrainingStageDone 标记阶段完成/未完成。
func (s *Store) SetTrainingStageDone(ctx context.Context, planID, stageID int64, done bool) error {
	_, err := s.pool.Exec(ctx, `update training_stage set done=$3,
		done_at = case when $3 then now() else null end
		where id=$1 and plan_id=$2`, stageID, planID, done)
	return wrapErr("更新带培阶段", err)
}

// GetTrainingPlanByID 按计划 id 取（管理端）。
func (s *Store) GetTrainingPlanByID(ctx context.Context, id int64) (*TrainingPlan, error) {
	var p TrainingPlan
	err := s.pool.QueryRow(ctx, `select p.id, p.user_id, p.title, coalesce(p.mentor,''), p.total_weeks,
		coalesce(p.target,''), p.status, coalesce(u.name,''), coalesce(d.name,''), u.dept_id
		from training_plan p left join org_user u on u.id=p.user_id
		left join org_dept d on d.id=u.dept_id where p.id=$1`, id).
		Scan(&p.ID, &p.UserID, &p.Title, &p.Mentor, &p.TotalWeeks, &p.Target, &p.Status, &p.UserName, &p.DeptName, &p.UserDeptID)
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapErr("查询带培计划", err)
	}
	rows, err := s.pool.Query(ctx, `select id, sort, coalesce(week,''), goal, tasks, done
		from training_stage where plan_id=$1 order by sort, id`, p.ID)
	if err != nil {
		return nil, wrapErr("查询带培阶段", err)
	}
	defer rows.Close()
	for rows.Next() {
		var st TrainingStage
		var tasks []byte
		if err := rows.Scan(&st.ID, &st.Sort, &st.Week, &st.Goal, &tasks, &st.Done); err != nil {
			return nil, err
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
	return &p, rows.Err()
}

// SetStageDoneByID 管理端标记阶段。
func (s *Store) SetStageDoneByID(ctx context.Context, stageID int64, done bool) error {
	tag, err := s.pool.Exec(ctx, `update training_stage set done=$2,
		done_at = case when $2 then now() else null end where id=$1`, stageID, done)
	if err != nil {
		return wrapErr("更新带培阶段", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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

// ListTrainingPlansScoped 按部门/师傅过滤（管理端与师傅端共用）。
func (s *Store) ListTrainingPlansScoped(ctx context.Context, deptID *int64, mentor string, limit, offset int) ([]TrainingPlan, int, error) {
	where := ` where (($1::bigint is not null and u.dept_id=$1) or ($2<>'' and p.mentor=$2))`
	rows, err := s.pool.Query(ctx, `select p.id, p.user_id, p.title, coalesce(p.mentor,''), p.total_weeks,
		coalesce(p.target,''), p.status, coalesce(u.name,''), coalesce(d.name,''), u.dept_id,
		(select count(*) from training_stage st where st.plan_id=p.id),
		(select count(*) from training_stage st where st.plan_id=p.id and st.done)
		from training_plan p left join org_user u on u.id=p.user_id
		left join org_dept d on d.id=u.dept_id`+where+`
		order by p.id desc limit $3 offset $4`, deptID, mentor, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询带培计划", err)
	}
	defer rows.Close()
	var out []TrainingPlan
	for rows.Next() {
		var p TrainingPlan
		if err := rows.Scan(&p.ID, &p.UserID, &p.Title, &p.Mentor, &p.TotalWeeks, &p.Target,
			&p.Status, &p.UserName, &p.DeptName, &p.UserDeptID, &p.StageTotal, &p.StageDone); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	var total int
	err = s.pool.QueryRow(ctx, `select count(*) from training_plan p
		left join org_user u on u.id=p.user_id`+where, deptID, mentor).Scan(&total)
	return out, total, wrapErr("统计带培计划", err)
}

// ListTrainingPlans 管理端：带培计划列表。
func (s *Store) ListTrainingPlans(ctx context.Context, limit, offset int) ([]TrainingPlan, int, error) {
	rows, err := s.pool.Query(ctx, `select p.id, p.user_id, p.title, coalesce(p.mentor,''), p.total_weeks,
		coalesce(p.target,''), p.status, coalesce(u.name,''), coalesce(d.name,''),
		(select count(*) from training_stage st where st.plan_id=p.id),
		(select count(*) from training_stage st where st.plan_id=p.id and st.done)
		from training_plan p left join org_user u on u.id=p.user_id
		left join org_dept d on d.id=u.dept_id
		order by p.id desc limit $1 offset $2`, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询带培计划", err)
	}
	defer rows.Close()
	var out []TrainingPlan
	for rows.Next() {
		var p TrainingPlan
		if err := rows.Scan(&p.ID, &p.UserID, &p.Title, &p.Mentor, &p.TotalWeeks, &p.Target,
			&p.Status, &p.UserName, &p.DeptName, &p.StageTotal, &p.StageDone); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	var total int
	err = s.pool.QueryRow(ctx, `select count(*) from training_plan`).Scan(&total)
	return out, total, wrapErr("统计带培计划", err)
}

var _ = model.LearnRecord{}
