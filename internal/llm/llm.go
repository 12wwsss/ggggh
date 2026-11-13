// Package llm 定义模型调用抽象。
//
// 方案约定：只依赖 OpenAI 兼容的 HTTP 接口（/v1/chat/completions、
// /v1/embeddings），不绑定推理引擎；开发期提供确定性桩实现，
// 改配置三项（provider/base_url/model）即可切到矿内离线模型，
// 业务代码不需要改动。
package llm

import "context"

// Message 是一条对话消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Chat 是对话模型接口。
type Chat interface {
	Chat(ctx context.Context, messages []Message) (string, error)
	Name() string
	Ping(ctx context.Context) error
}

// Embedder 是嵌入模型接口。
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dim() int
	Name() string
	Ping(ctx context.Context) error
}
