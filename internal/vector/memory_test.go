package vector

import (
	"context"
	"testing"
)

func doc(id, docID int64, text string, vec []float32) Doc {
	return Doc{ChunkID: id, DocID: docID, Content: text, Vec: Normalize(vec)}
}

func TestMemorySearchOrder(t *testing.T) {
	m := NewMemory(nil, nil)
	ctx := context.Background()
	_ = m.Upsert(ctx, []Doc{
		doc(1, 10, "出口畅通", []float32{0.1, 0.9}),
		doc(2, 10, "瓦斯超限撤人", []float32{0.9, 0.1}),
		doc(3, 11, "排水系统", []float32{0, 1}),
	})
	hits, err := m.Search(ctx, Normalize([]float32{1, 0}), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("topK 未生效，返回 %d 条", len(hits))
	}
	if hits[0].ChunkID != 2 {
		t.Fatalf("首位应为 chunk 2，实际 %d", hits[0].ChunkID)
	}
	if hits[0].Score < hits[1].Score {
		t.Fatal("结果未按相似度降序排列")
	}
}

func TestMemoryDeleteByDoc(t *testing.T) {
	m := NewMemory(nil, nil)
	ctx := context.Background()
	_ = m.Upsert(ctx, []Doc{
		doc(1, 10, "a", []float32{1, 0}),
		doc(2, 11, "b", []float32{1, 0}),
	})
	if err := m.DeleteByDoc(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if m.Size() != 1 {
		t.Fatalf("删除后应剩 1 条，实际 %d", m.Size())
	}
	hits, _ := m.Search(ctx, []float32{1, 0}, 5)
	if hits[0].ChunkID != 2 {
		t.Fatalf("残留了已删除文档的向量: %+v", hits)
	}
}

func TestMemoryUpsertReplaces(t *testing.T) {
	m := NewMemory(nil, nil)
	ctx := context.Background()
	_ = m.Upsert(ctx, []Doc{doc(1, 10, "old", []float32{0, 1})})
	_ = m.Upsert(ctx, []Doc{doc(1, 10, "new", []float32{1, 0})})
	if m.Size() != 1 {
		t.Fatalf("同 chunk 应覆盖而非追加，实际 %d 条", m.Size())
	}
	hits, _ := m.Search(ctx, []float32{1, 0}, 1)
	if hits[0].Content != "new" {
		t.Fatalf("内容未覆盖: %q", hits[0].Content)
	}
}

func TestMemoryReloadAndEmpty(t *testing.T) {
	ctx := context.Background()
	m := NewMemory(func(context.Context) ([]Doc, error) {
		return []Doc{doc(5, 20, "reloaded", []float32{1, 0})}, nil
	}, nil)
	if err := m.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if m.Size() != 1 {
		t.Fatalf("装载后应有 1 条，实际 %d", m.Size())
	}
	empty := NewMemory(nil, nil)
	if _, err := empty.Search(ctx, []float32{1, 0}, 3); err != ErrEmptyIndex {
		t.Fatalf("空索引应返回 ErrEmptyIndex，实际 %v", err)
	}
}

func TestNormalizeAndDot(t *testing.T) {
	v := Normalize([]float32{3, 4})
	if diff := v[0]*v[0] + v[1]*v[1] - 1; diff > 1e-5 || diff < -1e-5 {
		t.Fatalf("归一化后模长不为 1: %v", v)
	}
	if s := Dot([]float32{1, 0}, []float32{1, 0}); s != 1 {
		t.Fatalf("同向向量点积应为 1，实际 %f", s)
	}
	if s := Dot([]float32{1, 0}, []float32{0, 1}); s != 0 {
		t.Fatalf("正交向量点积应为 0，实际 %f", s)
	}
}
