package auth

import (
	"testing"
	"time"
)

func TestIssueAndParse(t *testing.T) {
	m := NewManager("secret", time.Hour)
	token, exp, err := m.Issue(7, "worker01", "张明", "employee", 3)
	if err != nil {
		t.Fatal(err)
	}
	if token == "" || exp.Before(time.Now()) {
		t.Fatal("令牌签发异常")
	}
	claims, err := m.Parse(token)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if claims.UserID != 7 || claims.Role != "employee" || claims.DeptID != 3 {
		t.Fatalf("载荷不一致: %+v", claims)
	}
}

func TestParseRejectsTamperedToken(t *testing.T) {
	m := NewManager("secret", time.Hour)
	token, _, _ := m.Issue(1, "a", "A", "admin", 1)
	if _, err := m.Parse(token + "x"); err == nil {
		t.Fatal("被篡改的令牌必须拒绝")
	}
	if _, err := m.Parse("not-a-token"); err == nil {
		t.Fatal("非法令牌必须拒绝")
	}
}

func TestParseRejectsExpiredToken(t *testing.T) {
	m := NewManager("secret", -time.Minute)
	token, _, _ := m.Issue(1, "a", "A", "admin", 1)
	if _, err := m.Parse(token); err == nil {
		t.Fatal("过期令牌必须拒绝")
	}
}

func TestParseRejectsOtherSecret(t *testing.T) {
	issuer := NewManager("secret-a", time.Hour)
	verifier := NewManager("secret-b", time.Hour)
	token, _, _ := issuer.Issue(1, "a", "A", "admin", 1)
	if _, err := verifier.Parse(token); err == nil {
		t.Fatal("不同密钥签发的令牌必须拒绝")
	}
}

func TestPasswordHash(t *testing.T) {
	hash, err := HashPassword("Admin@123456")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "Admin@123456") {
		t.Fatal("正确口令校验失败")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("错误口令不应通过")
	}
	if hash == "Admin@123456" {
		t.Fatal("口令必须散列存储")
	}
}

func TestScopes(t *testing.T) {
	s := Build([]Policy{
		{Role: "admin", Resource: "doc", Action: "read", Scope: "all"},
		{Role: "employee", Resource: "qa", Action: "read", Scope: "self"},
	})
	if sc, ok := s.Scope("admin", "doc", "read"); !ok || sc != "all" {
		t.Fatalf("管理员文档范围应为 all，实际 %q ok=%v", sc, ok)
	}
	if s.Allowed("employee", "doc", "write") {
		t.Fatal("员工不应有文档写权限")
	}
	if sc, ok := s.Scope("employee", "qa", "read"); !ok || sc != "self" {
		t.Fatalf("员工问答范围应为 self，实际 %q ok=%v", sc, ok)
	}
	if s.Allowed("nobody", "qa", "read") {
		t.Fatal("未知角色不应放行")
	}
}
