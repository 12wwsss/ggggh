package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// exportAudit 导出审计日志 CSV，筛选条件与列表一致。
func (s *Server) exportAudit(c *gin.Context) {
	limit := 20000
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50000 {
			limit = n
		}
	}
	list, _, err := s.Store.ListAudit(c.Request.Context(), c.Query("resource"), c.Query("action"), limit, 0)
	if err != nil {
		fail(c, http.StatusInternalServerError, "导出失败")
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="audit-logs.csv"`)
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"时间", "操作人", "动作", "资源", "对象", "IP", "详情"})
	for _, l := range list {
		detail, _ := json.Marshal(l.Detail)
		_ = w.Write([]string{
			l.CreatedAt.Format("2006-01-02 15:04:05"),
			l.Username,
			l.Action,
			l.Resource,
			l.ResourceID,
			l.IP,
			string(detail),
		})
	}
	w.Flush()
}
