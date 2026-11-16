// Package config 负责加载运行配置。
//
// 配置来源优先级：环境变量 > configs/config.json > 内置默认值。
// 所有开关均可用环境变量覆盖，便于 systemd 单元文件注入。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Duration 支持在 JSON 配置里写 "90s"、"8h" 这类字符串，
// 也兼容直接写纳秒整数。环境变量始终用 time.ParseDuration 的格式。
type Duration time.Duration

// UnmarshalJSON 接受字符串或整数。
func (d *Duration) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if len(s) >= 2 && s[0] == '"' {
		parsed, err := time.ParseDuration(strings.Trim(s, `"`))
		if err != nil {
			return err
		}
		*d = Duration(parsed)
		return nil
	}
	if s == "null" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("时长格式错误: %s", s)
	}
	*d = Duration(n)
	return nil
}

// MarshalJSON 输出可读字符串。
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Duration(d).String() + `"`), nil
}

// D 返回标准库时长。
func (d Duration) D() time.Duration { return time.Duration(d) }

// LLM 描述对话模型服务。默认走内置桩实现，改 Provider=openai 即切换到
// OpenAI 兼容的 /v1/chat/completions，业务代码不需要改动。
type LLM struct {
	Provider    string   `json:"provider"`
	BaseURL     string   `json:"base_url"`
	APIKey      string   `json:"api_key"`
	Model       string   `json:"model"`
	Timeout     Duration `json:"timeout"`
	Temperature float64  `json:"temperature"`
	MaxTokens   int      `json:"max_tokens"`
}

// Embed 描述嵌入模型服务。
type Embed struct {
	Provider string   `json:"provider"`
	BaseURL  string   `json:"base_url"`
	APIKey   string   `json:"api_key"`
	Model    string   `json:"model"`
	Dim      int      `json:"dim"`
	Timeout  Duration `json:"timeout"`
}

// Vector 描述向量存储实现。
type Vector struct {
	Kind string `json:"kind"` // memory | pgvector
	Dim  int    `json:"dim"`
}

// Retrieval 描述检索与阈值策略。
type Retrieval struct {
	TopK            int     `json:"top_k"`
	MinScore        float64 `json:"min_score"`
	MaxContextChars int     `json:"max_context_chars"`
	// Hybrid 打开后，向量召回与 BM25 词法召回用 RRF 融合。
	Hybrid bool `json:"hybrid"`
	// LexicalMin 是词法召回的相对分门槛（相对本批最高分），
	// 用于放行“向量分不高但精确命中条款号/数字”的切片。
	LexicalMin float64 `json:"lexical_min"`
}

// Security 描述登录与限流策略。
type Security struct {
	MaxLoginFailures int `json:"max_login_failures"`
	LockMinutes      int `json:"lock_minutes"`
	RatePerMinute    int `json:"rate_per_minute"`
	// AllowedCIDRs 非空时只允许这些来源访问业务接口（/healthz、/metrics 豁免），
	// 用于“只对公司网络开放”的部署约束。
	AllowedCIDRs []string `json:"allowed_cidrs"`
}

// WeChat 是小程序登录配置；AppID 为空时走桩模式（本地与自测用）。
type WeChat struct {
	AppID  string `json:"app_id"`
	Secret string `json:"secret"`
}

// Task 描述后台任务表的保留策略。
type Task struct {
	// RetentionDays 为已结束任务的保留天数，超期由后台清理，避免任务表无限增长。
	RetentionDays int `json:"retention_days"`
}

// Logs 描述日志表的保留策略。问答留痕每天新增，审计日志需要更长追溯期，
// 两张表都只增不删，不设上限会一直涨。
type Logs struct {
	QALogDays int `json:"qa_log_days"`
	AuditDays int `json:"audit_days"`
}

// Config 是服务全部配置。
type Config struct {
	AppName    string    `json:"app_name"`
	HTTPAddr   string    `json:"http_addr"`
	DSN        string    `json:"dsn"`
	JWTSecret  string    `json:"jwt_secret"`
	JWTTTL     Duration  `json:"jwt_ttl"`
	StorageDir string    `json:"storage_dir"`
	LogLevel   string    `json:"log_level"`
	Bootstrap  bool      `json:"bootstrap"`
	TunnelAddr string    `json:"tunnel_addr"`
	WX         WeChat    `json:"wechat"`
	LLM        LLM       `json:"llm"`
	Embed      Embed     `json:"embed"`
	Vector     Vector    `json:"vector"`
	Retrieval  Retrieval `json:"retrieval"`
	Security   Security  `json:"security"`
	Task       Task      `json:"task"`
	Logs       Logs      `json:"logs"`
}

// Default 返回开发与内网试运行均可用的默认配置。
func Default() *Config {
	return &Config{
		AppName:    "煤矿安全员AI智能体平台",
		HTTPAddr:   "127.0.0.1:8080",
		DSN:        "postgres://postgres@127.0.0.1:55432/coalrag?sslmode=disable",
		JWTSecret:  "dev-only-change-me",
		JWTTTL:     Duration(8 * time.Hour),
		StorageDir: "storage",
		LogLevel:   "info",
		Bootstrap:  true,
		LLM: LLM{
			Provider:    "stub",
			BaseURL:     "http://127.0.0.1:8000/v1",
			Model:       "deepseek-r1:14b",
			Timeout:     Duration(90 * time.Second),
			Temperature: 0.2,
			MaxTokens:   1024,
		},
		Embed: Embed{
			Provider: "stub",
			BaseURL:  "http://127.0.0.1:8000/v1",
			Model:    "bge-m3",
			Dim:      1024,
			Timeout:  Duration(30 * time.Second),
		},
		Vector: Vector{Kind: "memory", Dim: 1024},
		// 阈值与嵌入模型绑定：桩实现用带符号哈希的字符 n-gram，标定区间约
		// (0.076, 0.085)，取 0.08；接入 bge-m3 等真实嵌入后必须用
		// `go run ./cmd/tune` 或评测集重新标定。
		Retrieval: Retrieval{TopK: 6, MinScore: 0.08, MaxContextChars: 6000, Hybrid: true, LexicalMin: 0.9},
		Security:  Security{MaxLoginFailures: 5, LockMinutes: 15, RatePerMinute: 600},
		Task:      Task{RetentionDays: 7},
		Logs:      Logs{QALogDays: 90, AuditDays: 180},
	}
}

// Load 读取配置。
func Load() (*Config, error) {
	cfg := Default()
	path := os.Getenv("COALRAG_CONFIG")
	if path == "" {
		path = "configs/config.json"
	}
	if b, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(b, cfg); err != nil {
			return nil, fmt.Errorf("解析配置文件 %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取配置文件 %s: %w", path, err)
	}

	applyEnv(cfg)

	if cfg.Embed.Dim <= 0 {
		return nil, fmt.Errorf("embed.dim 必须为正整数")
	}
	if cfg.Vector.Dim != cfg.Embed.Dim {
		return nil, fmt.Errorf("vector.dim(%d) 与 embed.dim(%d) 不一致", cfg.Vector.Dim, cfg.Embed.Dim)
	}
	if cfg.Vector.Kind != "memory" && cfg.Vector.Kind != "pgvector" {
		return nil, fmt.Errorf("vector.kind 只支持 memory 或 pgvector，当前为 %q", cfg.Vector.Kind)
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("jwt_secret 不能为空")
	}
	return cfg, nil
}

func applyEnv(c *Config) {
	str := func(key string, dst *string) {
		if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
			*dst = v
		}
	}
	integer := func(key string, dst *int) {
		if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
			if n, err := strconv.Atoi(v); err == nil {
				*dst = n
			}
		}
	}
	boolean := func(key string, dst *bool) {
		if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
			if b, err := strconv.ParseBool(v); err == nil {
				*dst = b
			}
		}
	}
	dur := func(key string, dst *Duration) {
		if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
			if d, err := time.ParseDuration(v); err == nil {
				*dst = Duration(d)
			}
		}
	}

	str("COALRAG_HTTP_ADDR", &c.HTTPAddr)
	str("COALRAG_DSN", &c.DSN)
	str("COALRAG_JWT_SECRET", &c.JWTSecret)
	str("COALRAG_STORAGE_DIR", &c.StorageDir)
	str("COALRAG_LOG_LEVEL", &c.LogLevel)
	dur("COALRAG_JWT_TTL", &c.JWTTTL)
	boolean("COALRAG_BOOTSTRAP", &c.Bootstrap)
	str("COALRAG_TUNNEL_ADDR", &c.TunnelAddr)
	c.WX.AppID = strings.TrimSpace(os.Getenv("COALRAG_WX_APPID"))
	c.WX.Secret = strings.TrimSpace(os.Getenv("COALRAG_WX_SECRET"))
	if v := strings.TrimSpace(os.Getenv("COALRAG_ALLOWED_CIDRS")); v != "" {
		for _, part := range strings.Split(v, ",") {
			if p := strings.TrimSpace(part); p != "" {
				c.Security.AllowedCIDRs = append(c.Security.AllowedCIDRs, p)
			}
		}
	}

	str("COALRAG_LLM_PROVIDER", &c.LLM.Provider)
	str("COALRAG_LLM_BASE_URL", &c.LLM.BaseURL)
	str("COALRAG_LLM_API_KEY", &c.LLM.APIKey)
	str("COALRAG_LLM_MODEL", &c.LLM.Model)
	dur("COALRAG_LLM_TIMEOUT", &c.LLM.Timeout)
	integer("COALRAG_LLM_MAX_TOKENS", &c.LLM.MaxTokens)

	str("COALRAG_EMBED_PROVIDER", &c.Embed.Provider)
	str("COALRAG_EMBED_BASE_URL", &c.Embed.BaseURL)
	str("COALRAG_EMBED_API_KEY", &c.Embed.APIKey)
	str("COALRAG_EMBED_MODEL", &c.Embed.Model)
	integer("COALRAG_EMBED_DIM", &c.Embed.Dim)

	str("COALRAG_VECTOR_KIND", &c.Vector.Kind)
	integer("COALRAG_VECTOR_DIM", &c.Vector.Dim)

	integer("COALRAG_TOP_K", &c.Retrieval.TopK)
	if v, ok := os.LookupEnv("COALRAG_MIN_SCORE"); ok && strings.TrimSpace(v) != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.Retrieval.MinScore = f
		}
	}
	integer("COALRAG_MAX_CONTEXT_CHARS", &c.Retrieval.MaxContextChars)
	boolean("COALRAG_HYBRID", &c.Retrieval.Hybrid)
	if v, ok := os.LookupEnv("COALRAG_LEXICAL_MIN"); ok && strings.TrimSpace(v) != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.Retrieval.LexicalMin = f
		}
	}

	integer("COALRAG_MAX_LOGIN_FAILURES", &c.Security.MaxLoginFailures)
	integer("COALRAG_LOCK_MINUTES", &c.Security.LockMinutes)
	integer("COALRAG_RATE_PER_MINUTE", &c.Security.RatePerMinute)

	integer("COALRAG_TASK_RETENTION_DAYS", &c.Task.RetentionDays)
	integer("COALRAG_QA_LOG_DAYS", &c.Logs.QALogDays)
	integer("COALRAG_AUDIT_LOG_DAYS", &c.Logs.AuditDays)
}
