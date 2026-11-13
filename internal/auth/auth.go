// Package auth 负责口令、JWT 与 RBAC 判定。
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Claims 是 JWT 载荷。
type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"uname"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	DeptID   int64  `json:"dept"`
	jwt.RegisteredClaims
}

// Manager 签发与校验令牌。
type Manager struct {
	secret []byte
	ttl    time.Duration
}

// NewManager 构造令牌管理器。
func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

// Issue 签发访问令牌。
func (m *Manager) Issue(userID int64, username, name, role string, deptID int64) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(m.ttl)
	claims := Claims{
		UserID:   userID,
		Username: username,
		Name:     name,
		Role:     role,
		DeptID:   deptID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			Issuer:    "coalrag",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(m.secret)
	return s, exp, err
}

// Parse 校验并解析令牌。
func (m *Manager) Parse(raw string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("不支持的签名算法 %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithIssuer("coalrag"), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, errors.New("令牌无效")
	}
	return claims, nil
}

// HashPassword 生成 bcrypt 口令散列。
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword 校验口令。
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// Policy 是一条 RBAC 规则。
type Policy struct {
	Role     string `json:"role"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Scope    string `json:"scope"`
}

// Scopes 是 role -> resource -> action -> scope 的判定表。
type Scopes map[string]map[string]map[string]string

// Build 由策略列表构建判定表。
func Build(policies []Policy) Scopes {
	s := Scopes{}
	for _, p := range policies {
		if _, ok := s[p.Role]; !ok {
			s[p.Role] = map[string]map[string]string{}
		}
		if _, ok := s[p.Role][p.Resource]; !ok {
			s[p.Role][p.Resource] = map[string]string{}
		}
		s[p.Role][p.Resource][p.Action] = p.Scope
	}
	return s
}

// Scope 返回角色对资源的访问范围；无权限时第二返回值为 false。
func (s Scopes) Scope(role, resource, action string) (string, bool) {
	if r, ok := s[role]; ok {
		if res, ok := r[resource]; ok {
			if sc, ok := res[action]; ok {
				return sc, true
			}
		}
	}
	return "", false
}

// Allowed 判定是否放行。
func (s Scopes) Allowed(role, resource, action string) bool {
	_, ok := s.Scope(role, resource, action)
	return ok
}
