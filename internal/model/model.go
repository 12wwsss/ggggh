// Package model 存放跨层共享的数据结构与枚举。
package model

import (
	"encoding/json"
	"time"
)

// 角色。
const (
	RoleAdmin      = "admin"
	RoleDeptLeader = "dept_leader"
	RoleSafety     = "safety"
	RoleEmployee   = "employee"
)

// 文档状态。
const (
	DocParsing       = "parsing"
	DocPendingReview = "pending_review"
	DocPublished     = "published"
	DocRejected      = "rejected"
	DocFailed        = "failed"
)

// 问题状态。
const (
	IssueReported  = "reported"
	IssueAccepted  = "accepted"
	IssueRejected  = "rejected"
	IssueRectified = "rectified"
	IssueVerified  = "verified"
	IssueClosed    = "closed"
)

// Dept 部门。
type Dept struct {
	ID        int64     `json:"id"`
	ParentID  *int64    `json:"parent_id"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Sort      int       `json:"sort"`
	CreatedAt time.Time `json:"created_at"`
}

// User 用户（不含口令散列）。
type User struct {
	ID           int64      `json:"id"`
	DeptID       *int64     `json:"dept_id"`
	DeptName     string     `json:"dept_name,omitempty"`
	Username     string     `json:"username"`
	Name         string     `json:"name"`
	Phone        string     `json:"phone,omitempty"`
	Post         string     `json:"post,omitempty"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	FailedCount  int        `json:"-"`
	LockedUntil  *time.Time `json:"-"`
	PasswordHash string     `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Doc 文档。
type Doc struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Category    string     `json:"category"`
	SourceName  string     `json:"source_name"`
	FileSize    int64      `json:"file_size"`
	SHA256      string     `json:"sha256"`
	Status      string     `json:"status"`
	Version     int        `json:"version"`
	ChunkCount  int        `json:"chunk_count"`
	UploaderID  *int64     `json:"uploader_id"`
	Uploader    string     `json:"uploader,omitempty"`
	ErrorMsg    string     `json:"error_msg,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// Chunk 条款切片。
type Chunk struct {
	ID        int64  `json:"id"`
	DocID     int64  `json:"doc_id"`
	DocTitle  string `json:"doc_title,omitempty"`
	ClauseNo  string `json:"clause_no"`
	Chapter   string `json:"chapter"`
	Seq       int    `json:"seq"`
	Title     string `json:"title,omitempty"`
	Content   string `json:"content"`
	CharCount int    `json:"char_count"`
	Part      int    `json:"part"`
	Published bool   `json:"published"`
}

// Citation 引用。
type Citation struct {
	DocID    int64   `json:"doc_id"`
	DocTitle string  `json:"doc_title"`
	ChunkID  int64   `json:"chunk_id"`
	ClauseNo string  `json:"clause_no"`
	Snippet  string  `json:"snippet"`
	Score    float64 `json:"score"`
}

// QALog 问答留痕。
type QALog struct {
	ID        int64      `json:"id"`
	UserID    *int64     `json:"user_id"`
	UserName  string     `json:"user_name,omitempty"`
	DeptID    *int64     `json:"dept_id"`
	Question  string     `json:"question"`
	Answer    string     `json:"answer"`
	Citations []Citation `json:"citations"`
	Answered  bool       `json:"answered"`
	TopScore  float64    `json:"top_score"`
	HitCount  int        `json:"hit_count"`
	LatencyMS int        `json:"latency_ms"`
	Model     string     `json:"model"`
	Feedback  string     `json:"feedback,omitempty"`
	IP        string     `json:"ip,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// LearnItem 学习条目。
type LearnItem struct {
	ID         int64     `json:"id"`
	Title      string    `json:"title"`
	Category   string    `json:"category"`
	Post       string    `json:"post,omitempty"`
	DeptID     *int64    `json:"dept_id,omitempty"`
	Content    string    `json:"content"`
	ClauseRefs []string  `json:"clause_refs"`
	DocID      *int64    `json:"doc_id,omitempty"`
	Sort       int       `json:"sort"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// LearnRecord 学习记录。
type LearnRecord struct {
	ID          int64      `json:"id"`
	ItemID      int64      `json:"item_id"`
	UserID      int64      `json:"user_id"`
	UserName    string     `json:"user_name,omitempty"`
	Status      string     `json:"status"`
	Progress    int        `json:"progress"`
	DurationSec int        `json:"duration_sec"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// QuizQuestion 小测题。
type QuizQuestion struct {
	ID         int64    `json:"id"`
	ItemID     *int64   `json:"item_id"`
	Stem       string   `json:"stem"`
	Options    []string `json:"options"`
	Answer     string   `json:"answer,omitempty"`
	Analysis   string   `json:"analysis,omitempty"`
	Difficulty string   `json:"difficulty"`
}

// ExamPaper 试卷。
type ExamPaper struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	DeptID *int64 `json:"dept_id,omitempty"`
	// DeptIDs 是发布范围，空数组表示全员。
	DeptIDs     []int64    `json:"dept_ids"`
	DeptNames   []string   `json:"dept_names,omitempty"`
	QuestionIDs []int64    `json:"question_ids"`
	TotalScore  int        `json:"total_score"`
	PassScore   int        `json:"pass_score"`
	DurationMin int        `json:"duration_min"`
	StartAt     *time.Time `json:"start_at,omitempty"`
	EndAt       *time.Time `json:"end_at,omitempty"`
	Status      string     `json:"status"`
	CreatedBy   *int64     `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	// MaxAttempts 是允许考试的次数，发布时设置，默认 1。
	MaxAttempts int `json:"max_attempts"`
}

// ExamRecord 考试记录。
type ExamRecord struct {
	ID          int64             `json:"id"`
	PaperID     int64             `json:"paper_id"`
	PaperTitle  string            `json:"paper_title,omitempty"`
	UserID      int64             `json:"user_id"`
	UserName    string            `json:"user_name,omitempty"`
	DeptName    string            `json:"dept_name,omitempty"`
	Answers     map[string]string `json:"answers"`
	Score       int               `json:"score"`
	Passed      bool              `json:"passed"`
	Status      string            `json:"status"`
	AttemptNo   int               `json:"attempt_no"`
	StartedAt   time.Time         `json:"started_at"`
	SubmittedAt *time.Time        `json:"submitted_at,omitempty"`
}

// BroadcastTask 宣贯任务。
type BroadcastTask struct {
	ID          int64      `json:"id"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	DocIDs      []int64    `json:"doc_ids"`
	DeptIDs     []int64    `json:"dept_ids"`
	PublisherID *int64     `json:"publisher_id,omitempty"`
	RequireAck  bool       `json:"require_ack"`
	Deadline    *time.Time `json:"deadline,omitempty"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	TotalCount  int        `json:"total_count"`
	AckCount    int        `json:"ack_count"`
	// MyStatus 是“当前用户”的签收状态：pending / read / ack。
	// 只在 /broadcast/mine 返回，管理员列表为空。
	MyStatus string `json:"my_status,omitempty"`
}

// BroadcastRecord 宣贯签收记录。
type BroadcastRecord struct {
	ID       int64      `json:"id"`
	TaskID   int64      `json:"task_id"`
	UserID   int64      `json:"user_id"`
	UserName string     `json:"user_name,omitempty"`
	DeptName string     `json:"dept_name,omitempty"`
	Status   string     `json:"status"`
	ReadAt   *time.Time `json:"read_at,omitempty"`
	AckAt    *time.Time `json:"ack_at,omitempty"`
}

// AuditLog 审计日志。
type AuditLog struct {
	ID         int64     `json:"id"`
	UserID     *int64    `json:"user_id"`
	Username   string    `json:"username"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id"`
	Detail     any       `json:"detail"`
	IP         string    `json:"ip"`
	UA         string    `json:"ua"`
	CreatedAt  time.Time `json:"created_at"`
}

// Task 后台任务。
type Task struct {
	ID          int64           `json:"id"`
	Kind        string          `json:"kind"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	LastError   string          `json:"last_error,omitempty"`
	RunAfter    time.Time       `json:"run_after"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Issue 问题（二期）。
type Issue struct {
	ID            int64      `json:"id"`
	Code          string     `json:"code"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	CateID        *int64     `json:"cate_id,omitempty"`
	CateName      string     `json:"cate_name,omitempty"`
	DeptID        *int64     `json:"dept_id,omitempty"`
	DeptName      string     `json:"dept_name,omitempty"`
	ReporterID    *int64     `json:"reporter_id,omitempty"`
	Reporter      string     `json:"reporter,omitempty"`
	AssigneeID    *int64     `json:"assignee_id,omitempty"`
	Assignee      string     `json:"assignee,omitempty"`
	Severity      string     `json:"severity"`
	Status        string     `json:"status"`
	Deadline      *time.Time `json:"deadline,omitempty"`
	Result        string     `json:"result,omitempty"`
	VerifyComment string     `json:"verify_comment,omitempty"`
	Overdue       bool       `json:"overdue"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
}

// IssueFlow 流转记录。
type IssueFlow struct {
	ID         int64     `json:"id"`
	IssueID    int64     `json:"issue_id"`
	Action     string    `json:"action"`
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	ActorID    *int64    `json:"actor_id"`
	Actor      string    `json:"actor,omitempty"`
	Comment    string    `json:"comment"`
	CreatedAt  time.Time `json:"created_at"`
}

// IssueAttachment 附件。
type IssueAttachment struct {
	ID         int64     `json:"id"`
	IssueID    int64     `json:"issue_id"`
	FileName   string    `json:"file_name"`
	FileSize   int64     `json:"file_size"`
	SHA256     string    `json:"sha256"`
	UploaderID *int64    `json:"uploader_id"`
	Kind       string    `json:"kind"`
	CreatedAt  time.Time `json:"created_at"`
}

// IssueCate 问题分类。
type IssueCate struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ParentID *int64 `json:"parent_id,omitempty"`
	Sort     int    `json:"sort"`
}
