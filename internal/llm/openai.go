package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"coalrag/internal/config"
)

// OpenAI 是 OpenAI 兼容的 HTTP 客户端，同时实现对话与嵌入。
type OpenAI struct {
	baseURL   string
	apiKey    string
	model     string
	dim       int
	timeout   time.Duration
	temp      float64
	maxTok    int
	client    *http.Client
	embedOnly bool
}

// NewOpenAIChat 构造对话客户端。
func NewOpenAIChat(c config.LLM) *OpenAI {
	return &OpenAI{
		baseURL: strings.TrimRight(c.BaseURL, "/"),
		apiKey:  c.APIKey,
		model:   c.Model,
		timeout: c.Timeout.D(),
		temp:    c.Temperature,
		maxTok:  c.MaxTokens,
		client:  &http.Client{Timeout: c.Timeout.D()},
	}
}

// NewOpenAIEmbedder 构造嵌入客户端。
func NewOpenAIEmbedder(c config.Embed) *OpenAI {
	return &OpenAI{
		baseURL:   strings.TrimRight(c.BaseURL, "/"),
		apiKey:    c.APIKey,
		model:     c.Model,
		dim:       c.Dim,
		timeout:   c.Timeout.D(),
		client:    &http.Client{Timeout: c.Timeout.D()},
		embedOnly: true,
	}
}

func (o *OpenAI) Name() string { return "openai-compatible:" + o.model }
func (o *OpenAI) Dim() int     { return o.dim }

func (o *OpenAI) do(ctx context.Context, path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求模型服务: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("模型服务返回 %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("解析模型响应: %w", err)
	}
	return nil
}

// Chat 调用 /v1/chat/completions。
func (o *OpenAI) Chat(ctx context.Context, messages []Message) (string, error) {
	payload := map[string]any{
		"model":       o.model,
		"messages":    messages,
		"stream":      false,
		"temperature": o.temp,
	}
	if o.maxTok > 0 {
		payload["max_tokens"] = o.maxTok
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := o.do(ctx, "/chat/completions", payload, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("模型未返回 choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// Embed 调用 /v1/embeddings。
func (o *OpenAI) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	payload := map[string]any{"model": o.model, "input": texts}
	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := o.do(ctx, "/embeddings", payload, &out); err != nil {
		return nil, err
	}
	vecs := make([][]float32, len(texts))
	for _, d := range out.Data {
		if d.Index >= 0 && d.Index < len(vecs) {
			vecs[d.Index] = d.Embedding
		}
	}
	for i, v := range vecs {
		if v == nil {
			return nil, fmt.Errorf("嵌入响应缺少第 %d 条结果", i)
		}
		if o.dim > 0 && len(v) != o.dim {
			return nil, fmt.Errorf("嵌入维度 %d 与配置 %d 不一致", len(v), o.dim)
		}
	}
	return vecs, nil
}

// Ping 通过 /models 探活。
func (o *OpenAI) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.baseURL+"/models", nil)
	if err != nil {
		return err
	}
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("模型服务探活返回 %d", resp.StatusCode)
	}
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
