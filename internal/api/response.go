// Package api 提供 HTTP 接口。
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"coalrag/internal/auth"
)

// resp 是统一响应体。
type resp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// page 是分页响应体。
type page struct {
	List     any `json:"list"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, resp{Code: 0, Message: "ok", Data: data})
}

func fail(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, resp{Code: status, Message: msg})
}

func paged(c *gin.Context, list any, total, p, size int) {
	c.JSON(http.StatusOK, resp{Code: 0, Message: "ok", Data: page{List: list, Total: total, Page: p, PageSize: size}})
}

func pageParams(c *gin.Context) (p, size, offset int) {
	p, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if p < 1 {
		p = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}
	return p, size, (p - 1) * size
}

func idParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		fail(c, http.StatusBadRequest, "id 参数无效")
		return 0, false
	}
	return id, true
}

// claims 返回当前登录用户声明。
func claims(c *gin.Context) *auth.Claims {
	v, ok := c.Get(ctxClaims)
	if !ok {
		return nil
	}
	cl, _ := v.(*auth.Claims)
	return cl
}

// scope 返回当前请求的数据范围。
func scopeOf(c *gin.Context) string {
	if v, ok := c.Get(ctxScope); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return "self"
}

// userDept 返回当前用户部门。
func userDept(c *gin.Context) (int64, bool) {
	if cl := claims(c); cl != nil && cl.DeptID > 0 {
		return cl.DeptID, true
	}
	return 0, false
}
