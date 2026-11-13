// userctl 是账号急救工具：不经过 HTTP 接口直接操作数据库，
// 用于管理员自己被锁、或忘记口令时恢复。
//
//	go run ./cmd/userctl list
//	go run ./cmd/userctl unlock -u admin
//	go run ./cmd/userctl reset -u admin -p 'NewPass@123'
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"coalrag/internal/auth"
	"coalrag/internal/config"
	"coalrag/internal/db"
	"coalrag/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	dsn := os.Getenv("COALRAG_DSN")
	if dsn == "" {
		dsn = config.Default().DSN
	}

	ctx := context.Background()
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		fatal("连接数据库失败: %v", err)
	}
	defer pool.Close()
	st := store.New(pool)

	switch cmd {
	case "list":
		if err := list(ctx, st); err != nil {
			fatal("%v", err)
		}
	case "unlock":
		fs := flag.NewFlagSet("unlock", flag.ExitOnError)
		user := fs.String("u", "", "账号")
		_ = fs.Parse(args)
		if *user == "" {
			fatal("缺少 -u 账号")
		}
		if err := unlock(ctx, st, *user); err != nil {
			fatal("%v", err)
		}
	case "reset":
		fs := flag.NewFlagSet("reset", flag.ExitOnError)
		user := fs.String("u", "", "账号")
		pass := fs.String("p", "", "新口令（至少 8 位）")
		_ = fs.Parse(args)
		if *user == "" || *pass == "" {
			fatal("缺少 -u 账号或 -p 新口令")
		}
		if err := reset(ctx, st, *user, *pass); err != nil {
			fatal("%v", err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `用法：
  userctl list                       列出账号与锁定状态
  userctl unlock -u <账号>            解除锁定并清零失败计数
  userctl reset  -u <账号> -p <口令>  重置口令（同时解锁）
环境变量 COALRAG_DSN 可覆盖数据库连接串。`)
}

func list(ctx context.Context, st *store.Store) error {
	users, _, err := st.ListUsers(ctx, nil, "", 500, 0)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\t账号\t姓名\t角色\t状态\t失败次数\t锁定至")
	now := time.Now()
	locked := 0
	for _, u := range users {
		lock := "—"
		if u.LockedUntil != nil && u.LockedUntil.After(now) {
			lock = u.LockedUntil.Format("2006-01-02 15:04:05")
			locked++
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%d\t%s\n",
			u.ID, u.Username, u.Name, u.Role, u.Status, u.FailedCount, lock)
	}
	_ = w.Flush()
	fmt.Printf("\n共 %d 个账号，其中锁定 %d 个。\n", len(users), locked)
	return nil
}

func unlock(ctx context.Context, st *store.Store, username string) error {
	u, err := st.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("账号 %s 不存在", username)
		}
		return err
	}
	if err := st.UnlockUser(ctx, u.ID); err != nil {
		return err
	}
	fmt.Printf("已解锁 %s（%s），失败计数清零。\n", u.Username, u.Name)
	return nil
}

func reset(ctx context.Context, st *store.Store, username, password string) error {
	if len([]rune(password)) < 8 {
		return fmt.Errorf("口令至少 8 位")
	}
	u, err := st.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("账号 %s 不存在", username)
		}
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	if err := st.UpdatePassword(ctx, u.ID, hash); err != nil {
		return err
	}
	fmt.Printf("已重置 %s（%s）的口令并解锁。\n", u.Username, u.Name)
	return nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
