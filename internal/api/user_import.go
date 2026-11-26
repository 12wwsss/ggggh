package api

import (
	"encoding/csv"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"coalrag/internal/auth"
	"coalrag/internal/model"
)

var (
	usernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{4,12}$`)
	phoneRe    = regexp.MustCompile(`^1\d{10}$`)
)

// importUsers 批量导入员工：CSV（UTF-8）
// 表头：工号,姓名,部门,岗位,初始口令,手机号,角色
// 部门支持名称或编码；角色支持中文（员工/区队长/安全科/平台管理员）或英文代码。
// ?preview=1 时只校验不写库。
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

	preview := c.Query("preview") == "1"
	depts, _ := s.Store.ListDepts(c.Request.Context())
	deptByName := map[string]int64{}
	for _, d := range depts {
		deptByName[d.Name] = d.ID
		if d.Code != "" {
			deptByName[d.Code] = d.ID
		}
	}
	posts, _ := s.Store.DistinctPosts(c.Request.Context())
	postSet := map[string]bool{}
	for _, p := range posts {
		postSet[p] = true
	}

	roleMap := map[string]string{
		"": "employee", "员工": model.RoleEmployee, "区队长": model.RoleDeptLeader,
		"安全科": model.RoleSafety, "平台管理员": model.RoleAdmin,
		model.RoleAdmin: model.RoleAdmin, model.RoleDeptLeader: model.RoleDeptLeader,
		model.RoleSafety: model.RoleSafety, model.RoleEmployee: model.RoleEmployee,
	}

	valid, invalid := 0, 0
	errs := []gin.H{}
	seen := map[string]int{}
	row := 1
	for {
		rec, err := reader.Read()
		if err == io.EOF {
			break
		}
		row++
		reason := ""
		if err != nil {
			reason = "格式错误"
		}
		username := get(rec, "工号")
		name := get(rec, "姓名")
		if reason == "" {
			switch {
			case username == "" || name == "":
				reason = "工号或姓名为空"
			case !usernameRe.MatchString(username):
				reason = "工号需 4-12 位字母/数字/下划线"
			case seen[username] > 0:
				reason = "文件内工号重复（第 " + itoa(seen[username]) + " 行已出现）"
			}
		}
		roleKey := get(rec, "角色")
		role, okRole := roleMap[roleKey]
		if reason == "" && !okRole {
			reason = "角色不合法（员工/区队长/安全科/平台管理员）"
		}
		var deptID *int64
		if reason == "" {
			if dn := get(rec, "部门"); dn != "" {
				id, ok := deptByName[dn]
				if !ok {
					reason = "部门不存在：" + dn
				} else {
					deptID = &id
				}
			} else if role == model.RoleEmployee || role == model.RoleDeptLeader || role == model.RoleSafety {
				reason = "员工/区队长/安全科必须填部门"
			}
		}
		if reason == "" {
			if p := get(rec, "岗位"); p != "" && !postSet[p] {
				reason = "岗位不存在：" + p + "（需与后台岗位配置一致）"
			}
		}
		if reason == "" {
			if ph := get(rec, "手机号"); ph != "" && !phoneRe.MatchString(ph) {
				reason = "手机号格式不正确"
			}
		}
		password := get(rec, "初始口令")
		if reason == "" && password != "" && len([]rune(password)) < 8 {
			reason = "初始口令至少 8 位"
		}
		if reason != "" {
			invalid++
			errs = append(errs, gin.H{"row": row, "username": username, "reason": reason})
			continue
		}
		seen[username] = row
		valid++
		if preview {
			continue
		}
		if password == "" {
			password = "Coal@123456"
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			invalid++
			valid--
			errs = append(errs, gin.H{"row": row, "username": username, "reason": "口令处理失败"})
			continue
		}
		_, err = s.Store.CreateUser(c.Request.Context(), &model.User{
			Username: username, Name: name, Phone: get(rec, "手机号"), Post: get(rec, "岗位"),
			Role: role, Status: "active", DeptID: deptID,
		}, hash)
		if err != nil {
			invalid++
			valid--
			errs = append(errs, gin.H{"row": row, "username": username, "reason": "工号已存在或写入失败"})
			continue
		}
		if valid >= 500 {
			break
		}
	}
	if preview {
		ok(c, gin.H{"preview": true, "valid": valid, "invalid": invalid, "errors": errs})
		return
	}
	ok(c, gin.H{"created": valid, "skipped": invalid, "errors": errs})
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [8]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
