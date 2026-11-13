package vector

import (
	"context"
	"sort"
	"sync"
)

// Memory 是进程内内存实现：启动时从 doc_embedding 装载，写入时同步落库。
type Memory struct {
	mu      sync.RWMutex
	items   []Doc
	index   map[int64]int
	reload  func(context.Context) ([]Doc, error)
	persist func(context.Context, []Doc) error
}

// NewMemory 构造内存索引。reload 用于启动装载，persist 用于落库。
func NewMemory(reload func(context.Context) ([]Doc, error), persist func(context.Context, []Doc) error) *Memory {
	return &Memory{index: map[int64]int{}, reload: reload, persist: persist}
}

// Kind 返回实现名。
func (m *Memory) Kind() string { return "memory" }

// Reload 重新装载全量索引。
func (m *Memory) Reload(ctx context.Context) error {
	docs, err := m.reload(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make([]Doc, 0, len(docs))
	m.index = make(map[int64]int, len(docs))
	for _, d := range docs {
		m.index[d.ChunkID] = len(m.items)
		m.items = append(m.items, d)
	}
	return nil
}

// Upsert 写入向量并落库。
func (m *Memory) Upsert(ctx context.Context, docs []Doc) error {
	if len(docs) == 0 {
		return nil
	}
	if m.persist != nil {
		if err := m.persist(ctx, docs); err != nil {
			return err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range docs {
		if i, ok := m.index[d.ChunkID]; ok {
			m.items[i] = d
			continue
		}
		m.index[d.ChunkID] = len(m.items)
		m.items = append(m.items, d)
	}
	return nil
}

// DeleteByDoc 删除某文档的向量。
func (m *Memory) DeleteByDoc(_ context.Context, docID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	kept := m.items[:0]
	for _, d := range m.items {
		if d.DocID == docID {
			delete(m.index, d.ChunkID)
			continue
		}
		kept = append(kept, d)
	}
	m.items = kept
	return nil
}

// Search 线性扫描求余弦相似度前 topK。
func (m *Memory) Search(_ context.Context, query []float32, topK int) ([]Hit, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.items) == 0 {
		return nil, ErrEmptyIndex
	}
	if topK <= 0 {
		topK = 6
	}
	hits := make([]Hit, 0, len(m.items))
	for _, d := range m.items {
		hits = append(hits, Hit{Doc: d, Score: Dot(query, d.Vec)})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].ChunkID < hits[j].ChunkID
		}
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits, nil
}

// Size 返回索引记录数。
func (m *Memory) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}
