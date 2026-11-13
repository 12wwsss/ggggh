// Package vector 定义向量检索抽象与两种实现。
//
// 方案约定：默认内存实现，接入嵌入模型后替换为 pgvector；两者对同一套
// 检索用例必须给出一致结果（同为余弦相似度、同一 topK 排序规则）。
package vector

import (
	"context"
	"errors"
)

// Doc 是一条可检索的向量记录。
type Doc struct {
	ChunkID  int64
	DocID    int64
	DocTitle string
	ClauseNo string
	Chapter  string
	Content  string
	Seq      int
	Vec      []float32
}

// Hit 是检索结果。
type Hit struct {
	Doc
	Score float64
}

// Store 是检索接口。
type Store interface {
	// Upsert 写入或覆盖向量。
	Upsert(ctx context.Context, docs []Doc) error
	// DeleteByDoc 删除某文档的全部向量。
	DeleteByDoc(ctx context.Context, docID int64) error
	// Search 按余弦相似度返回前 topK 条。
	Search(ctx context.Context, query []float32, topK int) ([]Hit, error)
	// Reload 从持久化存储重新装载索引。
	Reload(ctx context.Context) error
	// Size 返回索引内记录数。
	Size() int
	// Kind 返回实现名。
	Kind() string
}

// ErrEmptyIndex 表示索引为空。
var ErrEmptyIndex = errors.New("检索索引为空")

// Dot 计算点积；调用方保证向量已归一化，点积即余弦相似度。
func Dot(a, b []float32) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var sum float32
	for i := 0; i < n; i++ {
		sum += a[i] * b[i]
	}
	return float64(sum)
}

// Normalize 就地做 L2 归一化，返回原切片。
func Normalize(v []float32) []float32 {
	var norm float32
	for _, x := range v {
		norm += x * x
	}
	if norm == 0 {
		return v
	}
	inv := 1 / sqrt32(norm)
	for i := range v {
		v[i] *= inv
	}
	return v
}

func sqrt32(x float32) float32 {
	// 牛顿迭代，避免引入额外依赖且保持精度足够。
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 20; i++ {
		z = 0.5 * (z + x/z)
	}
	return z
}
