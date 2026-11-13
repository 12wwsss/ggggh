// Package db 负责数据库连接与迁移执行。
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate 按文件名顺序执行迁移，每个文件一个事务，已执行过的跳过。
// 空库上启动即自动建表。
func Migrate(ctx context.Context, pool *pgxpool.Pool) (applied []string, err error) {
	if _, err = pool.Exec(ctx, `create table if not exists schema_migration (
		version text primary key,
		applied_at timestamptz not null default now()
	)`); err != nil {
		return nil, fmt.Errorf("创建迁移记录表: %w", err)
	}

	entries, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)

	for _, name := range entries {
		version := strings.TrimSuffix(strings.TrimPrefix(name, "migrations/"), ".sql")
		var exists bool
		if err = pool.QueryRow(ctx,
			`select exists(select 1 from schema_migration where version=$1)`, version).Scan(&exists); err != nil {
			return applied, fmt.Errorf("查询迁移 %s: %w", version, err)
		}
		if exists {
			continue
		}
		body, err := migrationFS.ReadFile(name)
		if err != nil {
			return applied, err
		}
		err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(body)); err != nil {
				return fmt.Errorf("执行迁移 %s: %w", version, err)
			}
			_, err := tx.Exec(ctx, `insert into schema_migration(version) values($1)`, version)
			return err
		})
		if err != nil {
			return applied, err
		}
		applied = append(applied, version)
	}
	return applied, nil
}
