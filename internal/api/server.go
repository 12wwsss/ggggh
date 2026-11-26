package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"coalrag/internal/auth"
	"coalrag/internal/config"
	"coalrag/internal/metrics"
	"coalrag/internal/service"
	"coalrag/internal/store"
	"coalrag/internal/wx"
)

// Server 组装路由与依赖。
type Server struct {
	Cfg         *config.Config
	Store       *store.Store
	Svc         *service.Service
	Auth        *auth.Manager
	Wx          *wx.Client
	Scopes      atomic.Pointer[auth.Scopes]
	Log         *slog.Logger
	Engine      *gin.Engine
	Static      http.Handler
	Metrics     *metrics.Registry
	limiter     *limiter
	allowedNets []*net.IPNet
	started     time.Time
}

// SetStatic 挂载前端静态资源（go:embed 的 dist 目录）。
func (s *Server) SetStatic(h http.Handler) { s.Static = h }

func (s *Server) policy() auth.Scopes {
	p := s.Scopes.Load()
	if p == nil {
		return auth.Scopes{}
	}
	return *p
}

// SetPolicy 热更新 RBAC 表。
func (s *Server) SetPolicy(p auth.Scopes) { s.Scopes.Store(&p) }

// New 构造 HTTP 服务。
func New(cfg *config.Config, st *store.Store, svc *service.Service, mgr *auth.Manager,
	logger *slog.Logger, policies auth.Scopes) *Server {

	s := &Server{
		Cfg:     cfg,
		Store:   st,
		Svc:     svc,
		Auth:    mgr,
		Log:     logger,
		Engine:  gin.New(),
		Metrics: metrics.New(),
		limiter: newLimiter(cfg.Security.RatePerMinute),
		started: time.Now(),
	}
	s.SetPolicy(policies)
	for _, raw := range cfg.Security.AllowedCIDRs {
		_, block, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err != nil {
			logger.Warn("来源网段配置无效，已忽略", "cidr", raw, "err", err)
			continue
		}
		s.allowedNets = append(s.allowedNets, block)
	}
	if len(s.allowedNets) > 0 {
		logger.Info("已启用来源网段白名单", "cidrs", cfg.Security.AllowedCIDRs)
	}

	s.Engine.Use(s.recoverMiddleware, s.traceMiddleware, s.corsMiddleware, s.rateLimitMiddleware, s.metricsMiddleware)

	s.Engine.GET("/healthz", s.healthz)
	s.Engine.GET("/readyz", s.readyz)
	s.Engine.GET("/metrics", s.metricsHandler)

	v1 := s.Engine.Group("/api/v1", s.cidrMiddleware)

	// 小程序兼容层（原 Node 代理的 /api/*）：先接健康检查与问答
	compat := s.Engine.Group("/api", s.cidrMiddleware)
	compat.GET("/health", s.compatHealth)
	compat.POST("/chat", s.compatChat)
	compat.POST("/login", s.compatLogin)
	compat.POST("/password/first", s.compatFirstPassword)
	compat.GET("/me", s.compatMe)
	compat.GET("/profile", s.compatProfile)
	compat.POST("/profile", s.compatSaveProfile)
	compat.GET("/quiz/stats", s.compatQuizStatsReal)
	compat.POST("/quiz", s.compatQuizSubmit)
	compat.GET("/qgroups", s.compatQuestionGroups)
	compat.GET("/questions", s.compatQuestionBank)
	compat.GET("/exams", s.compatExamList)
	compat.POST("/exam/start", s.compatExamStart)
	compat.POST("/exam/submit", s.compatExamSubmit)
	compat.POST("/learn/read", s.compatLearnRead)
	compat.GET("/wrongbook", s.compatWrongList)
	compat.GET("/training", s.compatTraining)
	compat.POST("/wrongbook", s.compatWrongSet)
	compat.GET("/exam/records", s.compatExamRecords)
	compat.GET("/exam/records/:id", s.compatExamReview)
	compat.GET("/faq", s.compatFAQ)
	compat.POST("/upload", s.compatUpload)
	compat.POST("/issue", s.compatCreateIssue)
	compat.GET("/issues", s.compatListIssues)
	compat.GET("/issues/stats", s.compatIssueStats)
	compat.POST("/issue/handle", s.compatHandleIssue)
	compat.POST("/issue/rectify", s.compatRectify)
	compat.POST("/issue/resubmit", s.compatResubmit)
	compat.GET("/notifications", s.compatNotifications)
	compat.GET("/messages", s.compatMessages)
	compat.POST("/messages/read", s.compatMessageRead)
	compat.GET("/issue/flow", s.compatIssueFlow)
	compat.GET("/users", s.compatUsers)
	s.Engine.GET("/uploads/:name", s.serveUpload)
	s.Engine.GET("/media/:name", s.serveMedia)
	compat.GET("/options", s.compatOptions)
	compat.GET("/culture", s.compatCulture)
	compat.GET("/content", s.compatContentList)
	compat.GET("/content/:id", s.compatContentDetail)
	compat.GET("/notices", s.compatNoticeList)
	compat.POST("/notice/read", s.compatNoticeAck)
	compat.GET("/learn/items", s.compatLearnItems)
	compat.GET("/learn/items/:id", s.compatLearnItemDetail)
	compat.POST("/learn/items/:id/progress", s.compatLearnProgress)
	compat.GET("/materials", s.compatMaterials)
	compat.GET("/risks", s.compatRisks)
	compat.GET("/employee/summary", s.compatEmployeeSummary)

	// 登录不需要令牌
	v1.POST("/auth/login", s.login)
	v1.POST("/auth/wx-login", s.wxLogin)
	v1.POST("/auth/wx-bind", s.wxBind)
	v1.POST("/auth/ticket", s.ticketLogin)

	authed := v1.Group("", s.authMiddleware(), s.auditMiddleware)
	authed.GET("/auth/me", s.me)
	authed.POST("/auth/password", s.changePassword)

	// 报表
	authed.GET("/reports/learning", s.learningReport)
	authed.GET("/reports/wrong", s.wrongReport)
	authed.GET("/reports/issues", s.issueReport)

	// 导出
	authed.GET("/export/issues", s.exportIssues)
	authed.GET("/export/reports", s.exportReports)
	authed.GET("/export/exam", s.exportExamSheet)
	authed.GET("/export/audit", s.exportAuditSheet)
	authed.GET("/export/quiz", s.exportQuiz)
	authed.POST("/import/quiz", s.importQuiz)

	// 视频/资料库
	authed.GET("/media", s.listMedia)
	authed.POST("/media/upload", s.uploadMedia)
	authed.POST("/media/link", s.createMediaLink)
	authed.DELETE("/media/:id", s.deleteMedia)

	// 岗位风险点
	authed.GET("/risks", s.listRisks)
	authed.POST("/risks", s.createRisk)
	authed.DELETE("/risks/:id", s.deleteRisk)

	// 新员工带培（权限在 handler 内按角色/师傅判断）
	authed.POST("/issues/:id/reassign", s.require("issue", "write"), s.reassignIssue)
	authed.GET("/training/plans", s.listTrainingPlans)
	authed.POST("/training/plans", s.createTrainingPlan)
	authed.POST("/training/plans/batch", s.createTrainingPlansBatch)
	authed.POST("/training/plans/:id/mentor", s.changeTrainingMentor)
	authed.GET("/training/plans/:id", s.getTrainingPlan)
	authed.POST("/training/stages/:id", s.setTrainingStage)

	// 组织与权限
	authed.GET("/depts", s.require("dept", "read"), s.listDepts)
	authed.GET("/posts", s.require("user", "read"), s.listPosts)
	authed.POST("/posts", s.require("user", "write"), s.createPost)
	authed.PUT("/posts/:id", s.require("user", "write"), s.updatePost)
	authed.DELETE("/posts/:id", s.require("user", "write"), s.deletePost)
	authed.POST("/depts", s.require("dept", "write"), s.createDept)
	authed.PUT("/depts/:id", s.require("dept", "write"), s.updateDept)
	authed.GET("/users", s.require("user", "read"), s.listUsers)
	authed.POST("/users", s.require("user", "write"), s.createUser)
	authed.POST("/users/import", s.require("user", "write"), s.importUsers)
	authed.PUT("/users/:id", s.require("user", "write"), s.updateUser)
	authed.POST("/users/:id/password", s.require("user", "write"), s.resetPassword)
	authed.POST("/users/:id/unlock", s.require("user", "write"), s.unlockUser)
	authed.GET("/roles", s.require("user", "read"), s.listRoles)
	authed.PUT("/roles/:role", s.require("user", "write"), s.updateRole)

	// 文档
	authed.GET("/docs", s.require("doc", "read"), s.listDocs)
	authed.POST("/docs", s.require("doc", "write"), s.uploadDoc)
	authed.GET("/docs/:id", s.require("doc", "read"), s.getDoc)
	authed.GET("/docs/:id/chunks", s.require("doc", "read"), s.listDocChunks)
	authed.POST("/docs/:id/publish", s.require("doc", "write"), s.publishDoc)
	authed.GET("/docs/:id/versions", s.require("doc", "read"), s.listDocVersions)
	authed.GET("/docs/:id/versions/:version", s.require("doc", "read"), s.getDocVersion)
	authed.GET("/docs/:id/diff", s.require("doc", "read"), s.diffDocVersions)
	authed.POST("/docs/:id/versions/:version/rollback", s.require("doc", "write"), s.rollbackDoc)
	authed.POST("/docs/:id/reject", s.require("doc", "write"), s.rejectDoc)
	authed.POST("/docs/:id/reparse", s.require("doc", "write"), s.reparseDoc)
	authed.DELETE("/docs/:id", s.require("doc", "write"), s.deleteDoc)
	authed.GET("/chunks/:id", s.require("doc", "read"), s.getChunk)
	authed.GET("/chunks/:id/text", s.require("doc", "read"), s.getChunkText)

	// 问答
	authed.POST("/qa/ask", s.require("qa", "write"), s.ask)
	authed.POST("/qa/search", s.require("qa", "read"), s.search)
	authed.GET("/qa/logs", s.require("qa", "read"), s.listQALogs)
	authed.GET("/qa/logs/:id", s.require("qa", "read"), s.getQALog)
	authed.POST("/qa/logs/:id/feedback", s.require("qa", "write"), s.qaFeedback)

	// 评测
	authed.POST("/eval/retrieval", s.require("eval", "write"), s.evalRetrieval)
	authed.POST("/eval/qa", s.require("eval", "write"), s.evalQA)
	authed.GET("/eval/reports", s.require("eval", "read"), s.listEvalReports)
	authed.GET("/eval/reports/:name", s.require("eval", "read"), s.getEvalReport)

	// 学习与小测
	authed.GET("/learn/items", s.require("learn", "read"), s.listLearnItems)
	authed.POST("/learn/items", s.require("learn", "write"), s.createLearnItem)
	authed.GET("/learn/items/:id", s.require("learn", "read"), s.getLearnItem)
	authed.GET("/learn/assignments", s.require("learn", "read"), s.listLearnAssignments)
	authed.POST("/learn/assignments", s.require("learn", "write"), s.createLearnAssignment)
	authed.DELETE("/learn/assignments/:id", s.require("learn", "write"), s.deleteLearnAssignment)
	authed.PUT("/learn/items/:id", s.require("learn", "write"), s.updateLearnItem)
	authed.POST("/learn/items/:id/progress", s.require("learn", "write"), s.saveLearnProgress)
	authed.GET("/learn/records", s.require("learn", "read"), s.listLearnRecords)
	authed.GET("/learn/export", s.require("learn", "read"), s.exportLearn)
	authed.GET("/quiz/items/:id/questions", s.require("learn", "read"), s.listQuizQuestions)
	authed.GET("/quiz/questions", s.require("learn", "read"), s.listAllQuestions)
	authed.POST("/quiz/questions", s.require("learn", "write"), s.createQuizQuestion)
	authed.POST("/quiz/questions/:id/status", s.require("learn", "write"), s.setQuizQuestionStatus)
	authed.POST("/quiz/items/:id/submit", s.require("learn", "write"), s.submitQuiz)

	// 考试
	authed.GET("/exam/papers", s.require("exam", "read"), s.listPapers)
	authed.POST("/exam/papers", s.require("exam", "write"), s.createPaper)
	authed.GET("/exam/papers/:id", s.require("exam", "read"), s.getPaper)
	authed.POST("/exam/papers/:id/publish", s.require("exam", "write"), s.publishPaper)
	authed.POST("/exam/papers/:id/start", s.require("exam", "write"), s.startExam)
	authed.POST("/exam/papers/:id/submit", s.require("exam", "write"), s.submitExam)
	authed.GET("/exam/papers/:id/record", s.require("exam", "read"), s.getExamRecord)
	authed.GET("/exam/records", s.require("exam", "read"), s.listExamRecords)
	authed.GET("/exam/export", s.require("exam", "read"), s.exportExam)

	// 宣贯
	authed.GET("/broadcast/tasks", s.require("broadcast", "read"), s.listBroadcastTasks)
	authed.POST("/broadcast/tasks", s.require("broadcast", "write"), s.createBroadcast)
	authed.POST("/broadcast/tasks/:id/publish", s.require("broadcast", "write"), s.publishBroadcast)
	authed.GET("/broadcast/tasks/:id/records", s.require("broadcast", "read"), s.listBroadcastRecords)
	authed.GET("/broadcast/tasks/:id/quiz-stats", s.require("broadcast", "read"), s.broadcastQuizStats)
	authed.POST("/broadcast/tasks/:id/ack", s.require("broadcast", "write"), s.ackBroadcast)
	authed.GET("/broadcast/mine", s.require("broadcast", "read"), s.myBroadcasts)

	// 二期问题闭环
	authed.GET("/issue-cates", s.require("issue", "read"), s.listIssueCates)
	authed.GET("/settings", s.require("user", "read"), s.getSettings)
	authed.PUT("/settings", s.require("user", "write"), s.updateSettings)
	authed.GET("/issues", s.require("issue", "read"), s.listIssues)
	authed.POST("/issues", s.require("issue", "write"), s.createIssue)
	authed.GET("/issues/stats", s.require("issue", "read"), s.issueStats)
	authed.GET("/issues/:id", s.require("issue", "read"), s.getIssue)
	authed.GET("/issues/:id/flow", s.require("issue", "read"), s.listIssueFlow)
	authed.POST("/issues/:id/accept", s.require("issue", "write"), s.acceptIssue)
	authed.POST("/issues/:id/reject", s.require("issue", "write"), s.rejectIssue)
	authed.POST("/issues/:id/rectify", s.require("issue", "write"), s.rectifyIssue)
	authed.POST("/issues/:id/verify", s.require("issue", "write"), s.verifyIssue)
	authed.POST("/issues/:id/close", s.require("issue", "write"), s.closeIssue)
	authed.GET("/issues/:id/attachments", s.require("issue", "read"), s.listIssueAttachments)
	authed.GET("/issues/:id/attachments.zip", s.require("issue", "read"), s.zipIssueAttachments)
	authed.GET("/issues/:id/attachments/:aid", s.require("issue", "read"), s.downloadIssueAttachment)
	authed.POST("/issues/:id/attachments", s.require("issue", "write"), s.uploadIssueAttachment)

	// 运维
	authed.GET("/dashboard", s.require("dashboard", "read"), s.dashboard)
	authed.GET("/audit", s.require("audit", "read"), s.listAudit)
	authed.GET("/audit/export", s.require("audit", "read"), s.exportAudit)
	authed.GET("/tasks", s.require("task", "read"), s.listTasks)
	authed.POST("/tasks/:id/retry", s.require("task", "write"), s.retryTask)

	s.Engine.NoRoute(s.noRoute)

	return s
}

// noRoute 把非 API 路径交给前端（SPA 回退），API 未知路径保持 JSON 404。
func (s *Server) noRoute(c *gin.Context) {
	p := c.Request.URL.Path
	if strings.HasPrefix(p, "/api/") || p == "/healthz" || p == "/readyz" {
		fail(c, http.StatusNotFound, "接口不存在")
		return
	}
	if s.Static != nil {
		s.Static.ServeHTTP(c.Writer, c.Request)
		return
	}
	c.String(http.StatusNotFound, "页面不存在")
}

// Handler 返回 gin 引擎。
func (s *Server) Handler() http.Handler { return s.Engine }

type check struct {
	Status  string  `json:"status"`
	Detail  string  `json:"detail,omitempty"`
	Latency float64 `json:"latency_ms,omitempty"`
}

func (s *Server) healthz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	checks := map[string]check{}

	start := time.Now()
	if err := s.Store.Ping(ctx); err != nil {
		checks["db"] = check{Status: "down", Detail: err.Error()}
	} else {
		checks["db"] = check{Status: "up", Latency: float64(time.Since(start).Microseconds()) / 1000}
	}

	start = time.Now()
	if err := s.Svc.Chat.Ping(ctx); err != nil {
		checks["llm"] = check{Status: "down", Detail: err.Error()}
	} else {
		checks["llm"] = check{Status: "up", Latency: float64(time.Since(start).Microseconds()) / 1000}
	}

	if s.Cfg.TunnelAddr == "" {
		checks["tunnel"] = check{Status: "not_configured", Detail: "未配置 COALRAG_TUNNEL_ADDR"}
	} else {
		start = time.Now()
		conn, err := net.DialTimeout("tcp", s.Cfg.TunnelAddr, 2*time.Second)
		if err != nil {
			checks["tunnel"] = check{Status: "down", Detail: err.Error()}
		} else {
			_ = conn.Close()
			checks["tunnel"] = check{Status: "up", Latency: float64(time.Since(start).Microseconds()) / 1000}
		}
	}

	status := http.StatusOK
	for _, v := range checks {
		if v.Status == "down" {
			status = http.StatusServiceUnavailable
		}
	}
	c.JSON(status, gin.H{
		"code":     0,
		"status":   map[bool]string{true: "ok", false: "degraded"}[status == http.StatusOK],
		"uptime_s": int(time.Since(s.started).Seconds()),
		"checks":   checks,
	})
}

func (s *Server) readyz(c *gin.Context) {
	if err := s.Store.Ping(c.Request.Context()); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 1, "message": "依赖未就绪", "detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ready"})
}
