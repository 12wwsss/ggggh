// tune 是检索阈值标定工具：在无向量库的情况下，直接对一份文档做
// 切分、嵌入与相似度排序，用可回答/不可回答两组问题估算合适的
// retrieval.min_score。
//
//	go run ./cmd/tune -doc testdata/煤矿安全规程节选.docx
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"coalrag/internal/config"
	"coalrag/internal/llm"
	"coalrag/internal/parser"
	"coalrag/internal/vector"
)

type probe struct {
	question string
	expect   string // 期望条款号，空表示不可回答
}

var probes = []probe{
	{"瓦斯浓度达到1.5%时怎么办？", "第一百三十六条"},
	{"检修电气设备要执行哪些制度？", "第四百四十二条"},
	{"敲帮问顶应该怎么做？", "第六条"},
	{"采煤工作面上下安全出口有什么要求？", "第七条"},
	{"停风时怎么处理？", "第一百三十八条"},
	{"接近老空区必须执行什么制度？", "第二百八十六条"},
	{"出现透水征兆怎么办？", "第二百八十八条"},
	{"主要排水泵房需要几台水泵？", "第二百八十七条"},
	{"提升装置需要哪些保护？", "第四百四十四条"},
	{"遇到断层破碎带怎么办？", "第八条"},
	{"从业人员上岗前需要什么？", "第四条"},
	{"瓦斯检查三对口指什么？", "第一百三十五条"},
	{"综采工作面采煤机滚筒截齿的更换周期是多少天？", ""},
	{"矿长每月下井次数的具体规定是多少？", ""},
	{"井下动火作业审批流程是怎样的？", ""},
	{"员工年休假天数如何计算？", ""},
}

type item struct {
	clause string
	text   string
	vec    []float32
}

func main() {
	docPath := flag.String("doc", "testdata/煤矿安全规程节选.docx", "规程文档")
	topN := flag.Int("top", 3, "每个问题打印前 N 条")
	dim := flag.Int("dim", 1024, "向量维度")
	flag.Parse()

	cfg := config.Default()
	cfg.Embed.Dim = *dim
	embedder := llm.NewStubEmbedder(cfg.Embed)

	blocks, err := (&parser.Docx{}).Parse(context.Background(), *docPath)
	if err != nil {
		fmt.Println("解析失败:", err)
		os.Exit(1)
	}
	chunks, err := parser.Split(blocks)
	if err != nil {
		fmt.Println("切分失败:", err)
		os.Exit(1)
	}

	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = strings.TrimSpace(c.Chapter + " " + c.ClauseNo + " " + c.Content)
	}
	vecs, err := embedder.Embed(context.Background(), texts)
	if err != nil {
		fmt.Println("嵌入失败:", err)
		os.Exit(1)
	}

	items := make([]item, len(chunks))
	for i, c := range chunks {
		items[i] = item{clause: c.ClauseNo, text: c.Content, vec: vector.Normalize(vecs[i])}
	}

	minAnswerable := 1.0
	maxUnanswerable := 0.0
	rankHits := 0

	for _, p := range probes {
		qv, _ := embedder.Embed(context.Background(), []string{p.question})
		q := vector.Normalize(qv[0])
		type scored struct {
			idx   int
			score float64
		}
		all := make([]scored, len(items))
		for i, it := range items {
			all[i] = scored{i, vector.Dot(q, it.vec)}
		}
		sort.Slice(all, func(a, b int) bool { return all[a].score > all[b].score })

		fmt.Printf("\nQ: %s\n", p.question)
		for k := 0; k < *topN && k < len(all); k++ {
			it := items[all[k].idx]
			mark := ""
			if it.clause == p.expect {
				mark = "  <- 命中"
			}
			fmt.Printf("   %.3f  %-8s %s%s\n", all[k].score, it.clause, trunc(it.text, 30), mark)
		}

		top1 := all[0].score
		if p.expect == "" {
			if top1 > maxUnanswerable {
				maxUnanswerable = top1
			}
		} else {
			if top1 < minAnswerable {
				minAnswerable = top1
			}
			for k := 0; k < 6 && k < len(all); k++ {
				if items[all[k].idx].clause == p.expect {
					rankHits++
					break
				}
			}
		}
	}

	fmt.Printf("\n可回答问题 top1 最低分: %.3f\n", minAnswerable)
	fmt.Printf("不可回答问题 top1 最高分: %.3f\n", maxUnanswerable)
	fmt.Printf("建议阈值区间: (%.3f, %.3f)\n", maxUnanswerable, minAnswerable)
	fmt.Printf("top6 命中率: %d/%d = %.2f\n", rankHits, countAnswerable(), float64(rankHits)/float64(countAnswerable()))
}

func countAnswerable() int {
	n := 0
	for _, p := range probes {
		if p.expect != "" {
			n++
		}
	}
	return n
}

func trunc(s string, n int) string {
	r := []rune(strings.ReplaceAll(s, "\n", ""))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
