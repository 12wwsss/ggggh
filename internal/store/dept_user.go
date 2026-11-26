package store

import (
	"context"
	"time"

	"coalrag/internal/auth"
	"coalrag/internal/model"
)

const (
	sqlUserByUsername = `select u.id, u.dept_id, coalesce(d.name,''), u.username, u.password_hash,
		u.name, coalesce(u.phone,''), coalesce(u.post,''), u.role, u.status,
		u.failed_attempts, u.locked_until, u.last_login_at, u.created_at
		from org_user u left join org_dept d on d.id = u.dept_id
		where u.username = $1`

	sqlUserByID = `select u.id, u.dept_id, coalesce(d.name,''), u.username, u.password_hash,
		u.name, coalesce(u.phone,''), coalesce(u.post,''), u.role, u.status,
		u.failed_attempts, u.locked_until, u.last_login_at, u.created_at
		from org_user u left join org_dept d on d.id = u.dept_id
		where u.id = $1`
)

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(r rowScanner) (*model.User, error) {
	var u model.User
	err := r.Scan(&u.ID, &u.DeptID, &u.DeptName, &u.Username, &u.PasswordHash,
		&u.Name, &u.Phone, &u.Post, &u.Role, &u.Status,
		&u.FailedCount, &u.LockedUntil, &u.LastLoginAt, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByUsername 按账号取用户（含口令散列）。
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, sqlUserByUsername, username))
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	return u, err
}

// GetUserByID 按主键取用户。
func (s *Store) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, sqlUserByID, id))
	if isNoRows(err) {
		return nil, ErrNotFound
	}
	if u != nil {
		u.PasswordHash = ""
	}
	return u, err
}

// ListUsers 分页查询用户。deptID 非空时只查该部门。
func (s *Store) ListUsers(ctx context.Context, deptID *int64, keyword string, limit, offset int) ([]model.User, int, error) {
	q := `select u.id, u.dept_id, coalesce(d.name,''), u.username, ''::text, u.name,
		coalesce(u.phone,''), coalesce(u.post,''), u.role, u.status,
		u.failed_attempts, u.locked_until, u.last_login_at, u.created_at, count(*) over() as total
		from org_user u left join org_dept d on d.id = u.dept_id
		where ($1::bigint is null or u.dept_id = $1)
		  and ($2 = '' or u.username ilike '%'||$2||'%' or u.name ilike '%'||$2||'%')
		order by u.id limit $3 offset $4`
	rows, err := s.pool.Query(ctx, q, deptID, keyword, limit, offset)
	if err != nil {
		return nil, 0, wrapErr("查询用户列表", err)
	}
	defer rows.Close()

	var out []model.User
	total := 0
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.DeptID, &u.DeptName, &u.Username, &u.PasswordHash,
			&u.Name, &u.Phone, &u.Post, &u.Role, &u.Status,
			&u.FailedCount, &u.LockedUntil, &u.LastLoginAt, &u.CreatedAt, &total); err != nil {
			return nil, 0, err
		}
		u.PasswordHash = ""
		out = append(out, u)
	}
	return out, total, rows.Err()
}

// CreateUser 新建用户。
func (s *Store) CreateUser(ctx context.Context, u *model.User, passwordHash string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `insert into org_user
		(dept_id, username, password_hash, name, phone, post, role, status, must_change_password)
		values ($1,$2,$3,$4,nullif($5,''),nullif($6,''),$7,$8,true) returning id`,
		u.DeptID, u.Username, passwordHash, u.Name, u.Phone, u.Post, u.Role, u.Status).Scan(&id)
	return id, wrapErr("新建用户", err)
}

// UpdateUser 更新用户基本资料。
func (s *Store) UpdateUser(ctx context.Context, u *model.User) error {
	tag, err := s.pool.Exec(ctx, `update org_user set dept_id=$2, name=$3, phone=nullif($4,''),
		post=nullif($5,''), role=$6, status=$7, updated_at=now() where id=$1`,
		u.ID, u.DeptID, u.Name, u.Phone, u.Post, u.Role, u.Status)
	if err != nil {
		return wrapErr("更新用户", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatePassword 修改口令并解锁（用户自己改，清除首改标记）。
func (s *Store) UpdatePassword(ctx context.Context, userID int64, hash string) error {
	tag, err := s.pool.Exec(ctx, `update org_user set password_hash=$2, failed_attempts=0,
		locked_until=null, must_change_password=false, updated_at=now() where id=$1`, userID, hash)
	if err != nil {
		return wrapErr("重置口令", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MustChangePassword 是否需要首次修改口令。
func (s *Store) MustChangePassword(ctx context.Context, userID int64) (bool, error) {
	var need bool
	err := s.pool.QueryRow(ctx, `select coalesce(must_change_password,false) from org_user where id=$1`, userID).Scan(&need)
	if isNoRows(err) {
		return false, ErrNotFound
	}
	return need, wrapErr("查询口令状态", err)
}

// ResetPassword 管理员重置口令（下次登录必须改）。
func (s *Store) ResetPassword(ctx context.Context, userID int64, hash string) error {
	tag, err := s.pool.Exec(ctx, `update org_user set password_hash=$2, failed_attempts=0,
		locked_until=null, must_change_password=true, updated_at=now() where id=$1`, userID, hash)
	if err != nil {
		return wrapErr("重置口令", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UnlockUser 解除锁定并清零失败计数。
func (s *Store) UnlockUser(ctx context.Context, userID int64) error {
	tag, err := s.pool.Exec(ctx, `update org_user set failed_attempts=0, locked_until=null,
		updated_at=now() where id=$1`, userID)
	if err != nil {
		return wrapErr("解锁用户", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LoginSucceeded 登录成功后清零失败计数。
func (s *Store) LoginSucceeded(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `update org_user set failed_attempts=0, locked_until=null,
		last_login_at=now() where id=$1`, userID)
	return wrapErr("更新登录状态", err)
}

// LoginFailed 登录失败计数加一，返回累计次数。
// 锁定时间由调用方在达到阈值时通过 LockUser 设置。
func (s *Store) LoginFailed(ctx context.Context, userID int64) (int, error) {
	var attempts int
	err := s.pool.QueryRow(ctx, `update org_user set failed_attempts=failed_attempts+1
		where id=$1 returning failed_attempts`, userID).Scan(&attempts)
	return attempts, wrapErr("记录登录失败", err)
}

// LockUser 锁定账号到指定时间。
func (s *Store) LockUser(ctx context.Context, userID int64, until time.Time) error {
	_, err := s.pool.Exec(ctx, `update org_user set locked_until=$2, updated_at=now() where id=$1`,
		userID, until)
	return wrapErr("锁定账号", err)
}

// ListPolicies 读取全部 RBAC 规则。
func (s *Store) ListPolicies(ctx context.Context) ([]auth.Policy, error) {
	rows, err := s.pool.Query(ctx, `select role, resource, action, scope from role_policy order by role, resource, action`)
	if err != nil {
		return nil, wrapErr("查询权限策略", err)
	}
	defer rows.Close()
	var out []auth.Policy
	for rows.Next() {
		var p auth.Policy
		if err := rows.Scan(&p.Role, &p.Resource, &p.Action, &p.Scope); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ReplacePolicies 覆盖某个角色的策略集合。
func (s *Store) ReplacePolicies(ctx context.Context, role string, list []auth.Policy) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `delete from role_policy where role=$1`, role); err != nil {
		return wrapErr("清理角色策略", err)
	}
	for _, p := range list {
		if _, err := tx.Exec(ctx, `insert into role_policy(role,resource,action,scope) values($1,$2,$3,$4)`,
			role, p.Resource, p.Action, p.Scope); err != nil {
			return wrapErr("写入角色策略", err)
		}
	}
	return tx.Commit(ctx)
}

// ---------- 部门 ----------

const sqlDeptList = `select id, parent_id, name, coalesce(code,''), sort, created_at
	from org_dept order by sort, id`

// ListDepts 列出全部部门。
func (s *Store) ListDepts(ctx context.Context) ([]model.Dept, error) {
	rows, err := s.pool.Query(ctx, sqlDeptList)
	if err != nil {
		return nil, wrapErr("查询部门", err)
	}
	defer rows.Close()
	var out []model.Dept
	for rows.Next() {
		var d model.Dept
		if err := rows.Scan(&d.ID, &d.ParentID, &d.Name, &d.Code, &d.Sort, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CreateDept 新建部门。
func (s *Store) CreateDept(ctx context.Context, d *model.Dept) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `insert into org_dept(parent_id,name,code,sort)
		values($1,$2,nullif($3,''),$4) returning id`, d.ParentID, d.Name, d.Code, d.Sort).Scan(&id)
	return id, wrapErr("新建部门", err)
}

// UpdateDept 更新部门。
func (s *Store) UpdateDept(ctx context.Context, d *model.Dept) error {
	tag, err := s.pool.Exec(ctx, `update org_dept set parent_id=$2, name=$3, code=nullif($4,''), sort=$5 where id=$1`,
		d.ID, d.ParentID, d.Name, d.Code, d.Sort)
	if err != nil {
		return wrapErr("更新部门", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountDeptUsers 统计部门在用人数。
func (s *Store) CountDeptUsers(ctx context.Context, deptID int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from org_user where dept_id=$1 and status='active'`, deptID).Scan(&n)
	return n, wrapErr("统计部门人数", err)
}
