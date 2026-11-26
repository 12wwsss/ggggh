package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"regexp"
	"sort"
	"strings"
	"time"

	"coalrag/internal/llm"
	"coalrag/internal/model"
	"coalrag/internal/search"
	"coalrag/internal/vector"
)

// FallbackAnswer 是无依据时的兜底回答。
const FallbackAnswer = "资料中未找到明确依据，建议查阅《煤矿安全规程》原文或向通风管理部门核实。"

var reCitation = regexp.MustCompile(`第[一二三四五六七八九十百千零〇两0-9]+条`)

// DegradedAnswer 是模型不可用但检索到条款时的回答，引用仍然可点开原文。
const DegradedAnswer = "模型服务暂不可用，先列出检索到的条款，请核对原文或联系安全科。"

// AskResult 是一次问答的结果。
type AskResult struct {
	Answered  bool             `json:"answered"`
	Fallback  bool             `json:"fallback"`
	Answer    string           `json:"answer"`
	Actions   []string         `json:"actions,omitempty"`
	Citations []model.Citation `json:"citations"`
	TopScore  float64          `json:"top_score"`
	HitCount  int              `json:"hit_count"`
	LatencyMS int              `json:"latency_ms"`
	Model     string           `json:"model"`
	LogID     int64            `json:"log_id,omitempty"`
}

// Retrieved 是一条带分数的检索结果。
type Retrieved struct {
	ChunkID  int64   `json:"chunk_id"`
	DocID    int64   `json:"doc_id"`
	DocTitle string  `json:"doc_title"`
	Chapter  string  `json:"chapter"`
	ClauseNo string  `json:"clause_no"`
	Content  string  `json:"content"`
	Seq      int     `json:"seq"`
	Score    float64 `json:"score"`
}

// Search 检索已发布条款。
//
// 默认走混合检索：向量召回与 BM25 词法召回各自取候选，用 RRF 融合排序。
// 阈值仍按向量相似度判断（保持既有标定），词法召回的条目按相对分放行，
// 这样“1.5%”“第一百三十六条”这类精确片段不会因为向量分数偏低被丢掉。
func (s *Service) Search(ctx context.Context, question string, topK int, minScore float64) ([]Retrieved, error) {
	if topK <= 0 {
		topK = s.Cfg.Retrieval.TopK
	}
	if minScore < 0 {
		minScore = s.Cfg.Retrieval.MinScore
	}
	vecs, err := s.Embedder.Embed(ctx, []string{question})
	if err != nil {
		return nil, fmt.Errorf("问题向量化失败: %w", err)
	}
	if len(vecs) != 1 {
		return nil, fmt.Errorf("嵌入模型返回条数异常")
	}
	if len(vecs[0]) != s.Cfg.Embed.Dim {
		return nil, fmt.Errorf("向量维度 %d 与配置 %d 不一致", len(vecs[0]), s.Cfg.Embed.Dim)
	}

	candidate := topK * 3
	if candidate < 12 {
		candidate = 12
	}
	vecHits, err := s.Vector.Search(ctx, vector.Normalize(vecs[0]), candidate)
	if err != nil && err != vector.ErrEmptyIndex {
		return nil, err
	}

	// 词法召回：索引为空（未装载）时退回纯向量。
	var lexHits []search.Hit
	if s.Lexical != nil && s.Cfg.Retrieval.Hybrid {
		lexHits = s.Lexical.Search(question, candidate)
	}
	if len(lexHits) == 0 && len(vecHits) == 0 {
		return nil, nil
	}

	hits := fuseHits(vecHits, lexHits, func(id int64) (*vector.Hit, bool) {
		chunk, err := s.Store.GetChunk(ctx, id)
		if err != nil || chunk == nil || !chunk.Published {
			return nil, false
		}
		return &vector.Hit{Doc: vector.Doc{
			ChunkID: chunk.ID, DocID: chunk.DocID, DocTitle: chunk.DocTitle,
			ClauseNo: chunk.ClauseNo, Chapter: chunk.Chapter, Content: chunk.Content, Seq: chunk.Seq,
		}}, true
	}, topK, minScore, s.Cfg.Retrieval.LexicalMin)

	out := make([]Retrieved, 0, len(hits))
	for _, h := range hits {
		out = append(out, Retrieved{
			ChunkID:  h.ChunkID,
			DocID:    h.DocID,
			DocTitle: h.DocTitle,
			Chapter:  h.Chapter,
			ClauseNo: h.ClauseNo,
			Content:  h.Content,
			Seq:      h.Seq,
			Score:    h.Score,
		})
	}
	return out, nil
}

// fuseHits 合并向量与词法召回：RRF 排序，并按“向量达标或词法相对分够高”放行。
// lookup 用于补齐只被词法命中的切片（生产从库里取，测试用内存表）。
func fuseHits(vecHits []vector.Hit, lexHits []search.Hit, lookup func(int64) (*vector.Hit, bool),
	topK int, minScore, lexicalMin float64) []vector.Hit {

	type cand struct {
		hit          vector.Hit
		vec, lex     float64
		rankV, rankL int
		matched      int
	}
	byID := make(map[int64]*cand, len(vecHits)+len(lexHits))
	for i := range vecHits {
		h := vecHits[i]
		byID[h.ChunkID] = &cand{hit: h, vec: h.Score, rankV: i + 1}
	}

	maxLex := 0.0
	for i := range lexHits {
		if lexHits[i].Score > maxLex {
			maxLex = lexHits[i].Score
		}
	}
	for i := range lexHits {
		id := lexHits[i].ID
		c := byID[id]
		if c == nil {
			if lookup == nil {
				continue
			}
			h, ok := lookup(id)
			if !ok {
				continue
			}
			c = &cand{hit: *h}
			byID[id] = c
		}
		c.lex = lexHits[i].Score
		c.rankL = i + 1
		c.matched = lexHits[i].Matched
	}

	type ranked struct {
		id    int64
		fused float64
		score float64
		lex   float64
	}
	list := make([]ranked, 0, len(byID))
	for id, c := range byID {
		fused := 0.0
		if c.rankV > 0 {
			fused += 1.0 / (60.0 + float64(c.rankV))
		}
		if c.rankL > 0 {
			fused += 1.0 / (60.0 + float64(c.rankL))
		}
		lexNorm := 0.0
		if maxLex > 0 {
			lexNorm = c.lex / maxLex
		}
		// 放行条件：向量达标；或整段词法精确命中（至少命中 2 个不同查询词、
		// 且相对分接近最高）同时向量分不算太差。后者专门补“条款号/数字”
		// 这类检索，但不会让“采煤”“必须”这种常用词把无关条款带进来。
		admitted := c.vec >= minScore ||
			(c.matched >= 2 && lexNorm >= lexicalMin && c.vec >= minScore*0.8)
		if !admitted {
			continue
		}
		// 对外一律报告余弦相似度：词法归一化分只在内部用于放行判断，
		// 混进 score 会让“阈值/分数”这类分析失去意义（上限恒为 1.0）。
		list = append(list, ranked{id: id, fused: fused, score: c.vec, lex: lexNorm})
	}

	// RRF 在两条候选表上是对称的：两个列表各排第 1、第 2 时分数相同。
	// 同分时优先词法命中更精确的一条，再按向量分、最后按 id。
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].fused != list[j].fused {
			return list[i].fused > list[j].fused
		}
		if list[i].lex != list[j].lex {
			return list[i].lex > list[j].lex
		}
		if list[i].score != list[j].score {
			return list[i].score > list[j].score
		}
		return list[i].id < list[j].id
	})
	// 同一份条款被重复上传成多份文档时，正文完全相同：只保留排名最高的一份，
	// 否则 topK 会被副本占满，答案里的依据也会重复。
	out := make([]vector.Hit, 0, topK)
	seenContent := make(map[uint64]struct{}, len(list))
	for _, r := range list {
		h := byID[r.id].hit
		key := contentKey(h.Content)
		if _, dup := seenContent[key]; dup {
			continue
		}
		seenContent[key] = struct{}{}
		h.Score = r.score
		out = append(out, h)
		if topK > 0 && len(out) >= topK {
			break
		}
	}
	return out
}

// Ask 执行检索增强问答：检索、阈值判断、上下文拼装、模型调用、
// 引用校验、兜底与留痕。
func (s *Service) Ask(ctx context.Context, user *model.User, question, ip string) (*AskResult, error) {
	start := time.Now()
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, fmt.Errorf("问题不能为空")
	}

	res := &AskResult{Model: s.Chat.Name()}

	hits, err := s.Search(ctx, question, s.Cfg.Retrieval.TopK, s.Cfg.Retrieval.MinScore)
	if err != nil && err != vector.ErrEmptyIndex {
		return nil, err
	}
	res.HitCount = len(hits)
	if len(hits) > 0 {
		res.TopScore = hits[0].Score
	}

	if len(hits) == 0 {
		res.Answered = false
		res.Fallback = true
		res.Answer = FallbackAnswer
		res.LatencyMS = int(time.Since(start).Milliseconds())
		s.persistLog(ctx, user, question, res, ip)
		return res, nil
	}

	prompt := buildPrompt(question, hits, s.Cfg.Retrieval.MaxContextChars)
	raw, err := s.Chat.Chat(ctx, []llm.Message{
		{Role: "system", Content: "你是煤矿安全员智能助手，只依据给定资料回答，并标注条款号。"},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		// 矿上 GPU 未起/模型重启时，不让整个问答变成 500：
		// 把检索到的条款直接给出来，员工至少能看到依据，比空白或报错有用。
		s.Log.Warn("模型调用失败，降级返回检索条款", "err", err, "hits", len(hits))
		res.Answered = false
		res.Fallback = true
		res.Answer = DegradedAnswer
		res.Citations = degradedCitations(hits, 3)
		res.Actions = nil
		res.LatencyMS = int(time.Since(start).Milliseconds())
		s.persistLog(ctx, user, question, res, ip)
		return res, nil
	}

	answer, cites, ok := verifyCitations(raw, hits)
	if ok {
		// 把“1. 2. 3.”这类动作行拆出来，前端渲染成可逐条执行的清单；
		// 正文只留说明，动作与依据分开，紧急场景不用从散文里自己抠。
		answer, res.Actions = splitActions(answer)
	}
	res.Answer = answer
	res.Citations = cites
	res.Answered = ok
	res.Fallback = !ok
	if !ok {
		res.Answer = FallbackAnswer
		res.Actions = nil
		res.Citations = []model.Citation{}
	}
	res.LatencyMS = int(time.Since(start).Milliseconds())
	s.persistLog(ctx, user, question, res, ip)
	return res, nil
}

// degradedCitations 取前 n 条检索结果作为引用，供前端展示与打开原文。
func degradedCitations(hits []Retrieved, n int) []model.Citation {
	if n <= 0 || n > len(hits) {
		n = len(hits)
	}
	out := make([]model.Citation, 0, n)
	for i := 0; i < n; i++ {
		h := hits[i]
		out = append(out, model.Citation{
			DocID: h.DocID, DocTitle: h.DocTitle, ChunkID: h.ChunkID,
			ClauseNo: h.ClauseNo, Snippet: snippetOf(h.Content, 120), Score: h.Score,
		})
	}
	return out
}

// reActionLine 匹配“1.”“1、”“1)”“①”开头的动作行。
var reActionLine = regexp.MustCompile(`^\s*(?:\d{1,2}\s*[.、)．]|[①②③④⑤⑥⑦⑧⑨⑩])\s*(.+)$`)

// splitActions 抽出编号动作行，返回剩余正文与动作列表。
// 少于两条不拆，避免把普通编号列表误当处置步骤。
func splitActions(text string) (string, []string) {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	actions := make([]string, 0, 4)
	for _, ln := range lines {
		if m := reActionLine.FindStringSubmatch(ln); m != nil {
			action := strings.TrimSpace(m[1])
			if action != "" {
				actions = append(actions, action)
				continue
			}
		}
		kept = append(kept, ln)
	}
	if len(actions) < 2 {
		return text, nil
	}
	body := strings.Join(kept, "\n")
	for strings.Contains(body, "\n\n\n") {
		body = strings.ReplaceAll(body, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(body), actions
}

func (s *Service) persistLog(ctx context.Context, user *model.User, question string, res *AskResult, ip string) {
	if user == nil {
		return
	}
	l := &model.QALog{
		UserID:    &user.ID,
		DeptID:    user.DeptID,
		Question:  question,
		Answer:    res.Answer,
		Citations: res.Citations,
		Answered:  res.Answered,
		TopScore:  res.TopScore,
		HitCount:  res.HitCount,
		LatencyMS: res.LatencyMS,
		Model:     res.Model,
		IP:        ip,
	}
	if l.Citations == nil {
		l.Citations = []model.Citation{}
	}
	id, err := s.Store.InsertQALog(ctx, l)
	if err != nil {
		s.Log.Error("写入问答留痕失败", "err", err)
		return
	}
	res.LogID = id
}

func buildPrompt(question string, hits []Retrieved, maxChars int) string {
	var b strings.Builder
	b.WriteString("你是煤矿安全员智能助手。请只依据下面给出的资料片段回答，禁止编造条款号。\n\n资料片段：\n")
	used := 0
	idx := 0
	for _, h := range hits {
		body := strings.TrimSpace(h.Content)
		if used+len([]rune(body)) > maxChars && idx > 0 {
			break
		}
		idx++
		used += len([]rune(body))
		clause := h.ClauseNo
		if clause == "" {
			clause = "（正文）"
		}
		fmt.Fprintf(&b, "【片段%d】文档：《%s》 条款：%s\n内容：%s\n", idx, h.DocTitle, clause, body)
	}
	b.WriteString("\n规则：\n1. 只能使用上述片段中的内容，禁止编造条款号；\n")
	b.WriteString("2. 若条文里有需要立即执行的动作，先用“1. 2. 3.”每行一条列出，一句话一条，动词开头；\n")
	b.WriteString("3. 动作清单之后再用一两句话说明；回答末尾另起一行写“依据：[第X条]”，多个条款用顿号分隔；\n")
	b.WriteString("4. 若片段不足以回答，直接回答“资料中未找到明确依据”。\n\n")
	fmt.Fprintf(&b, "问题：%s", question)
	return b.String()
}

// verifyCitations 校验回答中的条款号必须来自检索结果，
// 并返回清理后的答案与可反查原文的引用列表。
func verifyCitations(raw string, hits []Retrieved) (string, []model.Citation, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", nil, false
	}
	if strings.Contains(text, "资料中未找到明确依据") {
		return "", nil, false
	}

	valid := map[string]Retrieved{}
	for _, h := range hits {
		if h.ClauseNo == "" {
			continue
		}
		if _, ok := valid[h.ClauseNo]; !ok {
			valid[h.ClauseNo] = h
		}
	}

	found := reCitation.FindAllString(text, -1)
	kept := make([]string, 0, len(found))
	seen := map[string]bool{}
	for _, c := range found {
		if _, ok := valid[c]; ok && !seen[c] {
			seen[c] = true
			kept = append(kept, c)
		}
	}
	if len(kept) == 0 {
		return "", nil, false
	}

	cites := make([]model.Citation, 0, len(kept))
	for _, c := range kept {
		h := valid[c]
		cites = append(cites, model.Citation{
			DocID:    h.DocID,
			DocTitle: h.DocTitle,
			ChunkID:  h.ChunkID,
			ClauseNo: c,
			Snippet:  snippetOf(h.Content, 120),
			Score:    h.Score,
		})
	}
	return text, cites, true
}

func snippetOf(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "……"
}

// contentKey 用正文哈希识别“同一条款的多份副本”。
func contentKey(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.Join(strings.Fields(s), "")))
	return h.Sum64()
}
