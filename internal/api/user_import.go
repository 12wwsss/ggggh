package api

import (
	"encoding/csv"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/auth"
	"coalrag/internal/model"
)

// importUsers 批量导入员工：CSV（UTF-8）
// 表头：工号,姓名,部门,岗位,初始口令,手机号,角色
func (s *Server) importUsers(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "请选择 CSV 文件")
		return
	}
	if file.Size > 4<<20 {
		fail(c, http.StatusBadRequest, "文件不能超过 4MB")
		return
	}
	f, err := file.Open()
	if err != nil {
		fail(c, http.StatusBadRequest, "读取文件失败")
		return
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		fail(c, http.StatusBadRequest, "CSV 为空或格式不正确")
		return
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimSpace(strings.TrimPrefix(h, "\ufeff"))] = i
	}
	get := func(rec []string, name string) string {
		i, ok := idx[name]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}
	if _, ok := idx["工号"]; !ok {
		fail(c, http.StatusBadRequest, "缺少表头：工号")
		return
	}
	if _, ok := idx["姓名"]; !ok {
		fail(c, http.StatusBadRequest, "缺少表头：姓名")
		return
	}

	depts, _ := s.Store.ListDepts(c.Request.Context())
	deptByName := map[string]int64{}
	for _, d := range depts {
		deptByName[d.Name] = d.ID
	}

	created, skipped := 0, 0
	errs := []gin.H{}
	row := 1
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		row++
		if err != nil {
			skipped++
			errs = append(errs, gin.H{"row": row, "reason": "格式错误"})
			continue
		}
		username := get(rec, "工号")
		name := get(rec, "姓名")
		if username == "" || name == "" {
			skipped++
			errs = append(errs, gin.H{"row": row, "reason": "工号或姓名为空"})
			continue
		}
		role := get(rec, "角色")
		if role == "" {
			role = "employee"
		}
		if !validRole(role) {
			skipped++
			errs = append(errs, gin.H{"row": row, "username": username, "reason": "角色不合法"})
			continue
		}
		var deptID *int64
		if dn := get(rec, "部门"); dn != "" {
			id, ok := deptByName[dn]
			if !ok {
				skipped++
				errs = append(errs, gin.H{"row": row, "username": username, "reason": "部门不存在：" + dn})
				continue
			}
			deptID = &id
		}
		password := get(rec, "初始口令")
		if password == "" {
			password = "Coal@123456"
		}
		if len([]rune(password)) < 8 {
			skipped++
			errs = append(errs, gin.H{"row": row, "username": username, "reason": "初始口令至少 8 位"})
			continue
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			skipped++
			errs = append(errs, gin.H{"row": row, "username": username, "reason": "口令处理失败"})
			continue
		}
		_, err = s.Store.CreateUser(c.Request.Context(), &model.User{
			Username: username, Name: name, Phone: get(rec, "手机号"), Post: get(rec, "岗位"),
			Role: role, Status: "active", DeptID: deptID,
		}, hash)
		if err != nil {
			skipped++
			errs = append(errs, gin.H{"row": row, "username": username, "reason": "工号已存在或写入失败"})
			continue
		}
		created++
		if created >= 500 {
			break
		}
	}
	ok(c, gin.H{"created": created, "skipped": skipped, "errors": errs})
}
