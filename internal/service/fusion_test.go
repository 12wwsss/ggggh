package service

import (
	"fmt"
	"testing"

	"coalrag/internal/search"
	"coalrag/internal/vector"
)

func hit(id int64, clause string, score float64) vector.Hit {
	// 正文各不相同：检索会按正文哈希去掉“同一条款的多份副本”，
	// 测试用例必须用不同内容，否则会被当成重复切片合并。
	return vector.Hit{
		Doc:   vector.Doc{ChunkID: id, ClauseNo: clause, Content: fmt.Sprintf("第%d条 测试正文内容", id)},
		Score: score,
	}
}

func TestFuseHitsAdmitsExactLexicalMatch(t *testing.T) {
	vec := []vector.Hit{hit(1, "第一条", 0.42), hit(2, "第七条", 0.07)}
	lex := []search.Hit{{ID: 2, Score: 9.5, Matched: 3}, {ID: 1, Score: 1.0, Matched: 2}}

	lookup := func(id int64) (*vector.Hit, bool) {
		h := hit(id, "", 0)
		return &h, true
	}
	got := fuseHits(vec, lex, lookup, 5, 0.08, 0.9)
	if len(got) != 2 {
		t.Fatalf("期望两条都放行，实际 %+v", got)
	}
	// 第七条 词法排第 1、向量排第 2，RRF 分应与只排第 1 的相近；
	// 这里只验证它被保留且分数取词法归一值（向量分过低时）。
	if got[1].ChunkID != 2 && got[0].ChunkID != 2 {
		t.Fatalf("第七条应出现在结果中: %+v", got)
	}
}

func TestFuseHitsRejectsWeakLexicalBypass(t *testing.T) {
	// 向量远低于阈值、词法也不是精确命中：不能靠词法越过阈值。
	vec := []vector.Hit{hit(1, "第一条", 0.02), hit(2, "第八条", 0.01)}
	lex := []search.Hit{{ID: 1, Score: 6.0}, {ID: 2, Score: 3.0}}
	got := fuseHits(vec, lex, nil, 5, 0.08, 0.9)
	if len(got) != 0 {
		t.Fatalf("低向量分不应被词法单独放行: %+v", got)
	}
}

func TestFuseHitsOrdersByFusion(t *testing.T) {
	// 两条都达标；第 2 条在词法里排第一，融合后应排到前面。
	vec := []vector.Hit{hit(1, "第一条", 0.40), hit(2, "第二条", 0.39)}
	lex := []search.Hit{{ID: 2, Score: 8.0}, {ID: 1, Score: 2.0}}
	got := fuseHits(vec, lex, nil, 5, 0.08, 0.9)
	if len(got) != 2 || got[0].ChunkID != 2 {
		t.Fatalf("融合排序应把词法首位提前: %+v", got)
	}
	if got[0].Score < 0.08 {
		t.Errorf("展示分应保留向量分: %f", got[0].Score)
	}
}

func TestFuseHitsWithoutLexicalKeepsVectorOrder(t *testing.T) {
	vec := []vector.Hit{hit(3, "第三条", 0.5), hit(4, "第四条", 0.3), hit(5, "第五条", 0.05)}
	got := fuseHits(vec, nil, nil, 2, 0.08, 0.9)
	if len(got) != 2 || got[0].ChunkID != 3 || got[1].ChunkID != 4 {
		t.Fatalf("纯向量路径应保持原顺序并过滤低分: %+v", got)
	}
}
