package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 兼容层：小程序（原 Node 代理）使用 /api/*，响应约定 {ok:true,...}。
// 先落地健康检查与问答，其余接口按页面逐步补齐。

func (s *Server) compatHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"key":  "offline",
		"kb":   0,
		"llm":  true,
		"mode": "offline",
	})
}

type compatChatReq struct {
	Question string `json:"question"`
	Image    string `json:"image"`
}

func (s *Server) compatChat(c *gin.Context) {
	var req compatChatReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Question) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "message": "问题不能为空"})
		return
	}
	res, err := s.Svc.Ask(c.Request.Context(), nil, req.Question, c.ClientIP())
	if err != nil {
		s.Log.Error("兼容问答失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "message": "AI 服务异常，请稍后重试"})
		return
	}
	// 小程序问答页不渲染 actions 字段，这里把处置动作并入正文，
	// 避免出现只有“依据：[第一百三十六条]”而看不到具体步骤的情况。
	answer := strings.TrimSpace(res.Answer)
	if len(res.Actions) > 0 {
		lines := make([]string, 0, len(res.Actions)+2)
		if answer != "" {
			lines = append(lines, answer)
		}
		for i, a := range res.Actions {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, a))
		}
		answer = strings.Join(lines, "\n")
	}
	cites := make([]gin.H, 0, len(res.Citations))
	for _, ct := range res.Citations {
		title := strings.TrimSpace(ct.ClauseNo)
		if ct.DocTitle != "" {
			if title == "" {
				title = ct.DocTitle
			} else {
				title = title + " · " + ct.DocTitle
			}
		}
		cites = append(cites, gin.H{
			"id":        ct.ChunkID,
			"title":     title,
			"clause_no": ct.ClauseNo,
			"text":      ct.Snippet,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"answer":   answer,
		"cites":    cites,
		"grounded": res.Answered,
		"model":    res.Model,
	})
}
