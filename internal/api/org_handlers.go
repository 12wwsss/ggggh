package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/auth"
	"coalrag/internal/model"
)

func (s *Server) listDepts(c *gin.Context) {
	list, err := s.Store.ListDepts(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询部门失败")
		return
	}
	ok(c, list)
}

type deptReq struct {
	Name     string `json:"name" binding:"required"`
	Code     string `json:"code"`
	ParentID *int64 `json:"parent_id"`
	Sort     int    `json:"sort"`
}

func (s *Server) createDept(c *gin.Context) {
	var req deptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "部门名称必填")
		return
	}
	id, err := s.Store.CreateDept(c.Request.Context(), &model.Dept{
		Name: req.Name, Code: req.Code, ParentID: req.ParentID, Sort: req.Sort,
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "新建部门失败")
		return
	}
	ok(c, gin.H{"id": id})
}

func (s *Server) updateDept(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req deptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "部门名称必填")
		return
	}
	err := s.Store.UpdateDept(c.Request.Context(), &model.Dept{
		ID: id, Name: req.Name, Code: req.Code, ParentID: req.ParentID, Sort: req.Sort,
	})
	if err != nil {
		fail(c, http.StatusNotFound, "部门不存在")
		return
	}
	ok(c, gin.H{"updated": true})
}

func (s *Server) listUsers(c *gin.Context) {
	p, size, offset := pageParams(c)
	var dept *int64
	if sc := scopeOf(c); sc == "dept" {
		if d, okD := userDept(c); okD {
			dept = &d
		}
	}
	if v := c.Query("dept_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			dept = &id
		}
	}
	list, total, err := s.Store.ListUsers(c.Request.Context(), dept, strings.TrimSpace(c.Query("keyword")), size, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询用户失败")
		return
	}
	paged(c, list, total, p, size)
}

type userReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	DeptID   *int64 `json:"dept_id"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Post     string `json:"post"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

func validRole(r string) bool {
	return r == model.RoleAdmin || r == model.RoleDeptLeader || r == model.RoleEmployee
}

func (s *Server) createUser(c *gin.Context) {
	var req userReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数格式错误")
		return
	}
	if req.Username == "" || req.Name == "" {
		fail(c, http.StatusBadRequest, "账号与姓名必填")
		return
	}
	if !validRole(req.Role) {
		fail(c, http.StatusBadRequest, "角色只能是 admin、dept_leader、employee")
		return
	}
	if len([]rune(req.Password)) < 8 {
		fail(c, http.StatusBadRequest, "初始口令至少 8 位")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "口令处理失败")
		return
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	id, err := s.Store.CreateUser(c.Request.Context(), &model.User{
		Username: req.Username, Name: req.Name, Phone: req.Phone, Post: req.Post,
		Role: req.Role, Status: status, DeptID: req.DeptID,
	}, hash)
	if err != nil {
		fail(c, http.StatusConflict, "账号已存在或参数不合法")
		return
	}
	ok(c, gin.H{"id": id})
}

func (s *Server) updateUser(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req userReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数格式错误")
		return
	}
	if !validRole(req.Role) {
		fail(c, http.StatusBadRequest, "角色不合法")
		return
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	err := s.Store.UpdateUser(c.Request.Context(), &model.User{
		ID: id, Name: req.Name, Phone: req.Phone, Post: req.Post,
		Role: req.Role, Status: status, DeptID: req.DeptID,
	})
	if err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	ok(c, gin.H{"updated": true})
}

func (s *Server) resetPassword(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len([]rune(req.Password)) < 8 {
		fail(c, http.StatusBadRequest, "新口令至少 8 位")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "口令处理失败")
		return
	}
	if err := s.Store.UpdatePassword(c.Request.Context(), id, hash); err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	ok(c, gin.H{"updated": true})
}

// unlockUser 解除因口令连续错误造成的锁定。
func (s *Server) unlockUser(c *gin.Context) {
	id, okID := idParam(c)
	if !okID {
		return
	}
	if err := s.Store.UnlockUser(c.Request.Context(), id); err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	ok(c, gin.H{"unlocked": true})
}

func (s *Server) listRoles(c *gin.Context) {
	policies, err := s.Store.ListPolicies(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, "查询权限失败")
		return
	}
	matrix := map[string][]auth.Policy{}
	for _, p := range policies {
		matrix[p.Role] = append(matrix[p.Role], p)
	}
	ok(c, gin.H{
		"roles":  []string{model.RoleAdmin, model.RoleDeptLeader, model.RoleEmployee},
		"matrix": matrix,
	})
}

func (s *Server) updateRole(c *gin.Context) {
	role := c.Param("role")
	if !validRole(role) {
		fail(c, http.StatusBadRequest, "角色不合法")
		return
	}
	var req struct {
		Policies []auth.Policy `json:"policies"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "参数格式错误")
		return
	}
	for _, p := range req.Policies {
		if p.Action != "read" && p.Action != "write" {
			fail(c, http.StatusBadRequest, "动作只能是 read 或 write")
			return
		}
		if p.Scope != "all" && p.Scope != "dept" && p.Scope != "self" {
			fail(c, http.StatusBadRequest, "数据范围只能是 all、dept、self")
			return
		}
		p.Role = role
	}
	if err := s.Store.ReplacePolicies(c.Request.Context(), role, req.Policies); err != nil {
		fail(c, http.StatusInternalServerError, "保存权限失败")
		return
	}
	all, err := s.Store.ListPolicies(c.Request.Context())
	if err == nil {
		s.SetPolicy(auth.Build(all))
	}
	ok(c, gin.H{"updated": true})
}
