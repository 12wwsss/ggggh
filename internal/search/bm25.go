// Package search 提供轻量词法检索（BM25），与向量检索融合使用。
//
// 中文没有分词器时，用「ASCII 词 + 汉字二元组」切词：条款号、百分比、
// 数字能整体命中，中文按相邻两字建索引，足以补向量检索漏掉的精确词组
// （例如“1.5%”“第一百三十六条”“敲帮问顶”）。
package search

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"
)

const (
	k1 = 1.2
	b  = 0.75
)

// Hit 是一条词法检索结果。Matched 表示命中的不同查询词数，
// 用于区分“整句精确命中”和“只蹭到一个常用词”。
type Hit struct {
	ID      int64
	Score   float64
	Matched int
}

type doc struct {
	length int
	terms  map[string]int
}

// Index 是进程内 BM25 索引。
type Index struct {
	mu       sync.RWMutex
	docs     map[int64]*doc
	postings map[string]map[int64]int
	totalLen int
}

// NewIndex 构造空索引。
func NewIndex() *Index {
	return &Index{
		docs:     map[int64]*doc{},
		postings: map[string]map[int64]int{},
	}
}

// Clear 清空索引。
func (x *Index) Clear() {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.docs = map[int64]*doc{}
	x.postings = map[string]map[int64]int{}
	x.totalLen = 0
}

// Add 写入或覆盖一条文档。
func (x *Index) Add(id int64, text string) {
	terms := Tokenize(text)
	if len(terms) == 0 {
		return
	}
	tf := make(map[string]int, len(terms))
	for _, t := range terms {
		tf[t]++
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.removeLocked(id)
	x.docs[id] = &doc{length: len(terms), terms: tf}
	x.totalLen += len(terms)
	for term, n := range tf {
		if x.postings[term] == nil {
			x.postings[term] = map[int64]int{}
		}
		x.postings[term][id] = n
	}
}

// Remove 删除一条文档。
func (x *Index) Remove(id int64) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.removeLocked(id)
}

func (x *Index) removeLocked(id int64) {
	old, ok := x.docs[id]
	if !ok {
		return
	}
	for term := range old.terms {
		if p, ok := x.postings[term]; ok {
			delete(p, id)
			if len(p) == 0 {
				delete(x.postings, term)
			}
		}
	}
	x.totalLen -= old.length
	delete(x.docs, id)
}

// Size 返回文档数。
func (x *Index) Size() int {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return len(x.docs)
}

// Search 返回 BM25 得分最高的前 topK 条。
func (x *Index) Search(query string, topK int) []Hit {
	terms := unique(Tokenize(query))
	if len(terms) == 0 {
		return nil
	}

	x.mu.RLock()
	defer x.mu.RUnlock()

	n := float64(len(x.docs))
	if n == 0 {
		return nil
	}
	avgLen := float64(x.totalLen) / n
	if avgLen <= 0 {
		avgLen = 1
	}

	scores := map[int64]float64{}
	matched := map[int64]int{}
	for _, term := range terms {
		p := x.postings[term]
		if len(p) == 0 {
			continue
		}
		df := float64(len(p))
		idf := math.Log(1 + (n-df+0.5)/(df+0.5))
		for id, tf := range p {
			d := x.docs[id]
			denom := float64(tf) + k1*(1-b+b*float64(d.length)/avgLen)
			scores[id] += idf * float64(tf) * (k1 + 1) / denom
			matched[id]++
		}
	}
	if len(scores) == 0 {
		return nil
	}

	hits := make([]Hit, 0, len(scores))
	for id, s := range scores {
		hits = append(hits, Hit{ID: id, Score: s, Matched: matched[id]})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ID < hits[j].ID
		}
		return hits[i].Score > hits[j].Score
	})
	if topK > 0 && len(hits) > topK {
		hits = hits[:topK]
	}
	return hits
}

// Tokenize 切词：ASCII 词整体保留，汉字按二元组切分。
func Tokenize(s string) []string {
	s = strings.ToLower(s)
	out := make([]string, 0, len(s)/2)

	var ascii []rune
	var cjk []rune
	flushASCII := func() {
		if len(ascii) > 0 {
			out = append(out, string(ascii))
			ascii = ascii[:0]
		}
	}
	flushCJK := func() {
		if len(cjk) == 0 {
			return
		}
		if len(cjk) == 1 {
			out = append(out, string(cjk))
		}
		for i := 0; i+1 < len(cjk); i++ {
			out = append(out, string(cjk[i:i+2]))
		}
		cjk = cjk[:0]
	}

	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r):
			flushASCII()
			cjk = append(cjk, r)
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '%' || r == '.':
			flushCJK()
			ascii = append(ascii, r)
		default:
			flushASCII()
			flushCJK()
		}
	}
	flushASCII()
	flushCJK()
	return out
}

func unique(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
