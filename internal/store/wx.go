package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// GetWxBind 按 openid 查绑定关系。
func (s *Store) GetWxBind(ctx context.Context, openid string) (int64, error) {
	var userID int64
	err := s.pool.QueryRow(ctx, `select user_id from wx_bind where openid=$1`, openid).Scan(&userID)
	if isNoRows(err) {
		return 0, ErrNotFound
	}
	return userID, wrapErr("查询微信绑定", err)
}

// BindWx 写入 openid 与工号的绑定；同一用户重复绑定时替换旧 openid。
func (s *Store) BindWx(ctx context.Context, openid, unionid string, userID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `delete from wx_bind where user_id=$1`, userID); err != nil {
		return wrapErr("清理旧绑定", err)
	}
	if _, err := tx.Exec(ctx, `insert into wx_bind(openid, unionid, user_id, last_login_at)
		values($1, nullif($2,''), $3, now())
		on conflict (openid) do update set user_id=excluded.user_id, unionid=excluded.unionid, last_login_at=now()`,
		openid, unionid, userID); err != nil {
		return wrapErr("写入微信绑定", err)
	}
	return tx.Commit(ctx)
}

// TouchWxLogin 记录最近一次微信登录时间。
func (s *Store) TouchWxLogin(ctx context.Context, openid string) error {
	_, err := s.pool.Exec(ctx, `update wx_bind set last_login_at=now() where openid=$1`, openid)
	return wrapErr("更新微信登录时间", err)
}

// CreateLoginTicket 生成一次性登录票据（默认 60 秒）。
func (s *Store) CreateLoginTicket(ctx context.Context, userID int64, ttl time.Duration) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := hex.EncodeToString(buf)
	if _, err := s.pool.Exec(ctx, `insert into login_ticket(code, user_id, expires_at)
		values($1,$2, now() + $3::interval)`, code, userID, durationExpr(ttl)); err != nil {
		return "", wrapErr("生成登录票据", err)
	}
	return code, nil
}

// ConsumeLoginTicket 校验并作废票据，返回用户 ID。
func (s *Store) ConsumeLoginTicket(ctx context.Context, code string) (int64, error) {
	var userID int64
	err := s.pool.QueryRow(ctx, `update login_ticket set used_at=now()
		where code=$1 and used_at is null and expires_at > now()
		returning user_id`, code).Scan(&userID)
	if isNoRows(err) {
		return 0, ErrNotFound
	}
	return userID, wrapErr("核销登录票据", err)
}

// PurgeLoginTickets 清理过期票据，避免表无限增长。
func (s *Store) PurgeLoginTickets(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `delete from login_ticket where expires_at < now() - interval '1 hour'`)
	if err != nil {
		return 0, wrapErr("清理登录票据", err)
	}
	return tag.RowsAffected(), nil
}
