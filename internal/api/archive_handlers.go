package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"coalrag/internal/export"
	"coalrag/internal/model"
)

func learnStatusText(s string) string {
	switch s {
	case "completed":
		return "已完成"
	case "in_progress":
		return "学习中"
	default:
		return "未开始"
	}
}

func ackStatusText(s string) string {
	switch s {
	case "ack":
		return "已签收"
	case "read":
		return "已读"
	default:
		return "未读"
	}
}

func issueStatusText(s string) string {
	switch s {
	case model.IssueReported:
		return "已上报"
	case model.IssueAccepted:
		return "已受理"
	case model.IssueRejected:
		return "已驳回"
	case model.IssueRectified:
		return "已整改"
	case model.IssueVerified:
		return "已验收"
	case model.IssueClosed:
		return "已闭环"
	default:
		return s
	}
}

func examStatusText(s string) string {
	switch s {
	case "submitted":
		return "已交卷"
	case "timeout":
		return "超时交卷"
	default:
		return "进行中"
	}
}

func boolText(b bool) string {
	if b {
		return "是"
	}
	return "否"
}

// exportTrainingArchive 导出「一人一档」培训档案（多 sheet xlsx）。
// 培训员可导出全矿，安全科/管理员全部，区队长只能导本队。
func (s *Server) exportTrainingArchive(c *gin.Context) {
	cl := claims(c)
	switch cl.Role {
	case model.RoleAdmin, model.RoleSafety, model.RoleTrainer:
	case model.RoleDeptLeader:
	default:
		fail(c, http.StatusForbidden, "没有导出培训档案的权限")
		return
	}
	userID, okID := s.archiveUserID(c)
	if !okID {
		return
	}
	u, err := s.Store.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		fail(c, http.StatusNotFound, "用户不存在")
		return
	}
	if cl.Role == model.RoleDeptLeader {
		if u.DeptID == nil || cl.DeptID <= 0 || *u.DeptID != cl.DeptID {
			fail(c, http.StatusForbidden, "只能导出本队员工的档案")
			return
		}
	}
	a, err := s.Store.UserArchive(c.Request.Context(), userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "汇总档案失败")
		return
	}

	basic := [][]string{{
		a.User.Name, a.User.Username, a.User.DeptName, a.User.Post,
		roleLabel(a.User.Role), a.User.Status,
	}}
	learn := make([][]string, 0, len(a.Learn))
	for _, r := range a.Learn {
		learn = append(learn, []string{r.Title, r.Category, learnStatusText(r.Status),
			strconv.Itoa(r.Progress) + "%", strconv.Itoa(r.Minutes), r.Updated})
	}
	exams := make([][]string, 0, len(a.Exams))
	for _, r := range a.Exams {
		exams = append(exams, []string{r.Paper, strconv.Itoa(r.Score), boolText(r.Passed),
			examStatusText(r.Status), r.At})
	}
	bcs := make([][]string, 0, len(a.Broadcasts))
	for _, r := range a.Broadcasts {
		bcs = append(bcs, []string{r.Title, ackStatusText(r.Status), r.At})
	}
	wrongs := make([][]string, 0, len(a.Wrongs))
	for _, r := range a.Wrongs {
		wrongs = append(wrongs, []string{r.Stem, strconv.Itoa(r.Times), r.At})
	}
	issues := make([][]string, 0, len(a.Issues))
	for _, r := range a.Issues {
		issues = append(issues, []string{r.Code, r.Title, issueStatusText(r.Status), r.Post, r.At, r.Closed})
	}
	trainings := make([][]string, 0, len(a.Training))
	for _, r := range a.Training {
		trainings = append(trainings, []string{r.Title, r.Mentor, r.Status,
			strconv.Itoa(r.Weeks) + " 周", r.Stages, r.CreatedA, r.Notes})
	}

	sheets := []export.Sheet{
		{Name: "基本信息", Headers: []string{"姓名", "工号", "部门", "岗位", "角色", "状态"}, Rows: basic},
		{Name: "学习记录", Headers: []string{"学习条目", "分类", "状态", "进度", "学时", "更新时间"}, Rows: learn},
		{Name: "考试记录", Headers: []string{"试卷", "成绩", "是否通过", "状态", "交卷时间"}, Rows: exams},
		{Name: "宣贯签收", Headers: []string{"宣贯任务", "状态", "签收时间"}, Rows: bcs},
		{Name: "错题", Headers: []string{"题目", "答错次数", "最近答错"}, Rows: wrongs},
		{Name: "上报问题", Headers: []string{"编号", "标题", "状态", "岗位", "上报时间", "闭环时间"}, Rows: issues},
		{Name: "带培与小结", Headers: []string{"计划", "师傅", "状态", "周期", "阶段完成", "创建日期", "师傅小结"}, Rows: trainings},
	}
	xlsxHeader(c, "培训档案-"+u.Name)
	if err := export.XLSXBook(c.Writer, sheets); err != nil {
		s.Log.Error("导出培训档案失败", "err", err, "user_id", userID)
	}
}

// roleLabel 角色中文名，只用在本导出里。
func roleLabel(r string) string {
	switch r {
	case model.RoleAdmin:
		return "平台管理员"
	case model.RoleDeptLeader:
		return "区队长"
	case model.RoleSafety:
		return "安全科"
	case model.RoleTrainer:
		return "培训员"
	default:
		return "员工"
	}
}
