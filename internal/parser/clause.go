package parser

import (
	"fmt"
	"regexp"
	"strings"

	"coalrag/internal/model"
)

const (
	maxChunkRunes = 900
	minPartRunes  = 120
	// 无条款号的前言少于该字数时丢弃（封面、标题、颁布令等）。
	minPreambleRunes = 40
)

var (
	reChapter = regexp.MustCompile(`^第([一二三四五六七八九十百千零〇两0-9]+)章[\s　、.]*(.*)$`)
	reSection = regexp.MustCompile(`^第([一二三四五六七八九十百千零〇两0-9]+)节[\s　、.]*(.*)$`)
	reClause  = regexp.MustCompile(`^第([一二三四五六七八九十百千零〇两0-9]+)条[\s　、.]*(.*)$`)
	reTOC     = regexp.MustCompile(`^目[\s　]*录$|^附[\s　]*录.*$`)
	rePageNo  = regexp.MustCompile(`^[-—－\s]*\d{1,4}[-—－\s]*$`)
	// 目录行用点线引导页码，与正文的章标题、条款行区分开。
	reDotLeader = regexp.MustCompile(`[\.．·…]{4,}`)
)

// isTOCLine 判定目录行、页码行等需要丢弃的行。
func isTOCLine(s string) bool {
	if reTOC.MatchString(s) || rePageNo.MatchString(s) {
		return true
	}
	return reDotLeader.MatchString(s)
}

// Split 把文本块按“章/节/条”结构切分为条款切片。
//
// 规则：
//   - 识别“第X章”“第X节”作为层级上下文，写入 chunk.chapter；
//   - 识别“第X条”作为切片边界，条款号写入 chunk.clause_no；
//   - 超长条款按句读切成多段，共用同一 clause_no，part 递增；
//   - 目录行、纯页码行丢弃；首个条款之前的前言正文保留为无条款号切片。
func Split(blocks []Block) ([]model.Chunk, error) {
	var (
		chunks       []model.Chunk
		buf          []string
		curNo        string
		chapterTitle string
		sectionTitle string
	)

	curChapter := func() string {
		parts := make([]string, 0, 2)
		if chapterTitle != "" {
			parts = append(parts, chapterTitle)
		}
		if sectionTitle != "" {
			parts = append(parts, sectionTitle)
		}
		return strings.Join(parts, " / ")
	}

	flush := func() {
		text := strings.TrimSpace(strings.Join(buf, "\n"))
		buf = nil
		if text == "" {
			return
		}
		// 首个条款之前的封面、标题行通常很短，保留反而污染检索；
		// 有明显篇幅的前言正文（如颁布说明）才作为无条款号切片保留。
		if curNo == "" && len([]rune(text)) < minPreambleRunes {
			return
		}
		for _, c := range splitLong(text, curNo, curChapter()) {
			chunks = append(chunks, c)
		}
	}

	for _, b := range blocks {
		line := strings.TrimSpace(b.Text)
		if line == "" || isTOCLine(line) {
			continue
		}
		if m := reChapter.FindStringSubmatch(line); m != nil {
			flush()
			chapterTitle = line
			sectionTitle = ""
			curNo = ""
			continue
		}
		if m := reSection.FindStringSubmatch(line); m != nil {
			flush()
			sectionTitle = line
			curNo = ""
			continue
		}
		if m := reClause.FindStringSubmatch(line); m != nil {
			flush()
			curNo = "第" + m[1] + "条"
			rest := strings.TrimSpace(m[2])
			if rest != "" {
				buf = append(buf, rest)
			}
			continue
		}
		buf = append(buf, line)
	}
	flush()

	if len(chunks) == 0 {
		return nil, fmt.Errorf("文档中没有可切分的正文")
	}

	clauses := 0
	for _, c := range chunks {
		if c.ClauseNo != "" {
			clauses++
		}
	}
	if clauses == 0 {
		return nil, fmt.Errorf("未识别到“第X条”结构，请确认文档为规程条文格式")
	}

	for i := range chunks {
		chunks[i].Seq = i + 1
		if chunks[i].Title == "" {
			chunks[i].Title = titleOf(chunks[i].Content)
		}
	}
	return chunks, nil
}

func titleOf(content string) string {
	r := []rune(strings.ReplaceAll(content, "\n", ""))
	if len(r) > 24 {
		r = r[:24]
	}
	return string(r)
}

// splitLong 把超长条款切成多段，共用条款号，part 从 1 递增。
func splitLong(text, clauseNo, chapter string) []model.Chunk {
	runes := []rune(text)
	if len(runes) <= maxChunkRunes {
		return []model.Chunk{{
			ClauseNo:  clauseNo,
			Chapter:   chapter,
			Content:   text,
			CharCount: len(runes),
			Part:      1,
			Title:     titleOf(text),
		}}
	}

	var out []model.Chunk
	start := 0
	for start < len(runes) {
		end := start + maxChunkRunes
		if end >= len(runes) {
			end = len(runes)
		} else {
			// 回退到最近的句读边界，避免把一句话截断。
			cut := end
			for i := end; i > start+minPartRunes; i-- {
				if r := runes[i-1]; r == '。' || r == '；' || r == '！' || r == '？' || r == '\n' {
					cut = i
					break
				}
			}
			end = cut
		}
		part := runes[start:end]
		out = append(out, model.Chunk{
			ClauseNo:  clauseNo,
			Chapter:   chapter,
			Content:   strings.TrimSpace(string(part)),
			CharCount: len(part),
			Part:      len(out) + 1,
			Title:     titleOf(string(part)),
		})
		start = end
	}
	return out
}
