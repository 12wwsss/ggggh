package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"coalrag/internal/llm"
)

// BroadcastDraft 是制度宣贯的「解读要点」初稿。
// 建设方案六（6）要求「新制度、新规程发布后，平台自动生成解读要点」：
// 这里先让模型出初稿，再由管理员修改确认后发布。
type BroadcastDraft struct {
	Points  []string `json:"points"`
	Content string   `json:"content"`
	Source  string   `json:"source"` // llm=模型生成，rule=模型不可用时的规则提炼
	Model   string   `json:"model,omitempty"`
	Notice  string   `json:"notice,omitempty"`
}

const draftSystemPrompt = `你是煤矿安全培训助手。把制度条文改写成一线员工一看就懂的宣贯要点。
要求：
1. 只依据给定正文，不编造条款号、不加入正文里没有的要求；
2. 输出 4-6 条要点，每条不超过 40 个字，写成「做什么/不能做什么」的动作句；
3. 严格输出 JSON，不要代码块、不要解释：{"points":["…","…"],"content":"把要点连成一段通顺的宣贯正文"}`

// DraftBroadcast 生成宣贯解读要点初稿。
//
// 模型不可用（离线环境未起模型、桩实现返回兜底话术、输出不是 JSON）时，
// 退化为按句读提炼，保证功能可用、不阻塞发布流程；source 字段说明来源，
// 前端据此提示「规则提炼，建议人工补充」。
func (s *Service) DraftBroadcast(ctx context.Context, title, body string) (*BroadcastDraft, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("请先填写制度正文")
	}
	if s.Chat != nil {
		msgs := []llm.Message{
			{Role: "system", Content: draftSystemPrompt},
			{Role: "user", Content: fmt.Sprintf("标题：%s\n正文：\n%s", strings.TrimSpace(title), body)},
		}
		out, err := s.Chat.Chat(ctx, msgs)
		if err == nil {
			if d, ok := parseDraftJSON(out); ok {
				d.Source = "llm"
				d.Model = s.Chat.Name()
				return d, nil
			}
		} else {
			s.Log.Warn("宣贯解读要点生成失败，改用规则提炼", "err", err)
		}
	}
	d := ruleDraft(title, body)
	d.Source = "rule"
	d.Notice = "模型未返回可用结果，已按正文提炼，请人工补充后再发布"
	if s.Chat != nil {
		d.Model = s.Chat.Name()
	}
	return d, nil
}

// parseDraftJSON 从模型输出里抠出 JSON；带代码块或前后说明时也能解析。
func parseDraftJSON(raw string) (*BroadcastDraft, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	// 桩实现的兜底话术不是要点，直接判失败。
	if strings.Contains(raw, FallbackAnswer) {
		return nil, false
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, false
	}
	var d BroadcastDraft
	if err := json.Unmarshal([]byte(raw[start:end+1]), &d); err != nil {
		return nil, false
	}
	points := make([]string, 0, len(d.Points))
	for _, p := range d.Points {
		if p = normalizePoint(p); p != "" {
			points = append(points, p)
		}
		if len(points) == 6 {
			break
		}
	}
	if len(points) < 2 {
		return nil, false
	}
	d.Points = points
	if strings.TrimSpace(d.Content) == "" {
		d.Content = strings.Join(points, "\n")
	}
	return &d, true
}

// ruleDraft 在无模型时按句读提炼要点：取最长的几句话，压缩到 42 字以内。
func ruleDraft(title, body string) *BroadcastDraft {
	body = strings.ReplaceAll(body, "\r", "")
	raw := strings.FieldsFunc(body, func(r rune) bool {
		return r == '。' || r == '；' || r == ';' || r == '\n'
	})
	points := make([]string, 0, 6)
	for _, p := range raw {
		if p = normalizePoint(p); p != "" {
			points = append(points, p)
			if len(points) == 6 {
				break
			}
		}
	}
	if len(points) < 2 {
		// 正文没有标点（例如一整段），按字数硬切，避免返回空要点。
		runes := []rune(strings.Join(strings.Fields(body), ""))
		for i := 0; i < len(runes) && len(points) < 6; i += 40 {
			end := i + 40
			if end > len(runes) {
				end = len(runes)
			}
			if p := normalizePoint(string(runes[i:end])); p != "" {
				points = append(points, p)
			}
		}
	}
	content := strings.Join(points, "\n")
	if strings.TrimSpace(title) != "" && content != "" {
		content = strings.TrimSpace(title) + "\n" + content
	}
	return &BroadcastDraft{Points: points, Content: content}
}

// reLeadingNo 匹配句首的编号：「1.」「1、」「(1)」「（一）」「一、」。
var reLeadingNo = regexp.MustCompile(`^\s*(?:[0-9]{1,2}[.、．)）]|[（(][0-9一二三四五六七八九十]{1,3}[）)][、.．]?|[一二三四五六七八九十]{1,3}[、.．])\s*`)

// normalizePoint 去掉序号/空白，超长截断到 42 字。
// 只按「编号」模式剥离前缀，不能简单 TrimLeft 掉「一二三…」，
// 否则「一切作业必须…」会被截成「切作业必须…」。
func normalizePoint(in string) string {
	s := strings.TrimSpace(in)
	s = reLeadingNo.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	if len([]rune(s)) < 6 {
		return ""
	}
	if n := []rune(s); len(n) > 42 {
		s = strings.TrimRight(string(n[:42]), "，、, ")
	}
	return s
}
