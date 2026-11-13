// Package store 是数据访问层。按方案约定，全部 SQL 只出现在本包内，
// 迁移到国产数据库时只需改动这里。
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound 表示记录不存在。
var ErrNotFound = errors.New("记录不存在")

// ErrConflict 表示状态冲突（并发或跨状态请求）。
var ErrConflict = errors.New("状态冲突")

// Store 持有连接池。
type Store struct {
	pool *pgxpool.Pool
}

// New 构造 Store。
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool 暴露连接池给需要事务编排的服务层。
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

// arr64 把 nil 切片规范化为空数组，避免写入 NOT NULL 的数组列时变成 NULL。
func arr64(v []int64) []int64 {
	if v == nil {
		return []int64{}
	}
	return v
}

func arrStr(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func wrapErr(op string, err error) error {
	if err == nil || isNoRows(err) {
		return err
	}
	return fmt.Errorf("%s: %w", op, err)
}

// Ping 探活。
func (s *Store) Ping(ctx context.Context) error {
	var one int
	return s.pool.QueryRow(ctx, `select 1`).Scan(&one)
}
