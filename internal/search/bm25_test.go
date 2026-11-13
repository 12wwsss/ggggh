package search

import "testing"

func TestTokenizeKeepsNumbersAndBigrams(t *testing.T) {
	tokens := Tokenize("瓦斯浓度达到1.5%时")
	has := func(want string) bool {
		for _, t := range tokens {
			if t == want {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"瓦斯", "斯浓", "1.5%", "浓度"} {
		if !has(want) {
			t.Errorf("缺少词元 %q，实际 %v", want, tokens)
		}
	}
}

func TestIndexSearchRanksExactPhraseFirst(t *testing.T) {
	idx := NewIndex()
	idx.Add(1, "进入作业地点前必须敲帮问顶，处理活矸危岩。")
	idx.Add(2, "采煤工作面上下安全出口必须保持畅通。")
	idx.Add(3, "瓦斯浓度达到 1.5% 时停止工作。")

	hits := idx.Search("敲帮问顶", 2)
	if len(hits) == 0 || hits[0].ID != 1 {
		t.Fatalf("应命中第 1 条，实际 %+v", hits)
	}

	hits = idx.Search("1.5%", 2)
	if len(hits) == 0 || hits[0].ID != 3 {
		t.Fatalf("数字应命中第 3 条，实际 %+v", hits)
	}
}

func TestIndexRemoveAndOverwrite(t *testing.T) {
	idx := NewIndex()
	idx.Add(1, "敲帮问顶")
	idx.Add(2, "安全出口")
	if idx.Size() != 2 {
		t.Fatalf("期望 2 条，实际 %d", idx.Size())
	}
	idx.Remove(1)
	if idx.Size() != 1 {
		t.Fatalf("删除后期望 1 条，实际 %d", idx.Size())
	}
	if hits := idx.Search("敲帮问顶", 3); len(hits) != 0 {
		t.Fatalf("删除后不应命中: %+v", hits)
	}
	idx.Add(2, "排水系统")
	hits := idx.Search("排水", 3)
	if len(hits) != 1 || hits[0].ID != 2 {
		t.Fatalf("覆盖后应按新内容命中: %+v", hits)
	}
	if hits := idx.Search("安全出口", 3); len(hits) != 0 {
		t.Fatalf("覆盖后旧内容不应命中: %+v", hits)
	}
}

func TestIndexEmptyQuery(t *testing.T) {
	idx := NewIndex()
	idx.Add(1, "瓦斯")
	if hits := idx.Search("，。", 3); len(hits) != 0 {
		t.Fatalf("空查询应返回空，实际 %+v", hits)
	}
}
