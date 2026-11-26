package store

import (
	"context"
)

// GetSetting 读取配置。
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.pool.QueryRow(ctx, `select coalesce(value,'') from app_setting where key=$1`, key).Scan(&v)
	if isNoRows(err) {
		return "", nil
	}
	return v, wrapErr("读取配置", err)
}

// SetSetting 写入配置。
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.pool.Exec(ctx, `insert into app_setting(key, value, updated_at) values ($1,$2,now())
		on conflict (key) do update set value=excluded.value, updated_at=now()`, key, value)
	return wrapErr("保存配置", err)
}
