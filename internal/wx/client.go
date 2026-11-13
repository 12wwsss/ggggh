// Package wx 封装微信小程序登录：用 code 换 openid。
//
// 未配置 AppID 时为桩模式，返回 "stub-<code>"，方便本地与自测联调；
// 正式环境配置 COALRAG_WX_APPID / COALRAG_WX_SECRET 后走真实接口。
package wx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client 是小程序登录客户端。
type Client struct {
	AppID  string
	Secret string
	HTTP   *http.Client
}

// New 构造客户端。
func New(appID, secret string) *Client {
	return &Client{
		AppID:  strings.TrimSpace(appID),
		Secret: strings.TrimSpace(secret),
		HTTP:   &http.Client{Timeout: 8 * time.Second},
	}
}

// Enabled 表示是否配置了真实凭据。
func (c *Client) Enabled() bool { return c.AppID != "" && c.Secret != "" }

// Code2Session 用小程序登录 code 换 openid。
func (c *Client) Code2Session(ctx context.Context, code string) (openid, unionid string, err error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", "", fmt.Errorf("code 不能为空")
	}
	if !c.Enabled() {
		return "stub-" + code, "", nil
	}

	q := url.Values{}
	q.Set("appid", c.AppID)
	q.Set("secret", c.Secret)
	q.Set("js_code", code)
	q.Set("grant_type", "authorization_code")
	u := "https://api.weixin.qq.com/sns/jscode2session?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("请求微信接口: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", "", fmt.Errorf("解析微信响应: %w", err)
	}
	if out.ErrCode != 0 {
		return "", "", fmt.Errorf("微信返回 %d: %s", out.ErrCode, out.ErrMsg)
	}
	if out.OpenID == "" {
		return "", "", fmt.Errorf("微信未返回 openid")
	}
	return out.OpenID, out.UnionID, nil
}
