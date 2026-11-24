package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// broadcastQuizStats 宣贯配套答题统计（后台）。
func (s *Server) broadcastQuizStats(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "参数不正确")
		return
	}
	attempts, users, avgPct, passRate, rows, err := s.Store.BroadcastQuizStats(c.Request.Context(), id)
	if err != nil {
		fail(c, http.StatusInternalServerError, "统计失败")
		return
	}
	ok(c, gin.H{
		"attempts": attempts, "users": users,
		"avg_pct": int(avgPct + 0.5), "pass_rate": int(passRate + 0.5),
		"rows": rows,
	})
}
