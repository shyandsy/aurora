package loginguard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// serve 挂上中间件 h 跑一次 POST /login,返回状态码 + handler 是否被放行到达。
func serve(h gin.HandlerFunc, clientIP string) (status int, reached bool) {
	r := gin.New()
	r.Use(h)
	r.POST("/login", func(c *gin.Context) { reached = true; c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = clientIP + ":50000" // 无 XFF,ClientIP 取 RemoteAddr
	r.ServeHTTP(w, req)
	return w.Code, reached
}

// TestMiddlewareBlocksLockedIP IP 已被锁 → 中间件在 handler 前 429、不放行。
func TestMiddlewareBlocksLockedIP(t *testing.T) {
	ctx := context.Background()
	g := newG(newFake(), hardLockPolicy()) // IPFailLimit=2
	for i := 0; i < 3; i++ {               // 3 > 2 → 锁 IP
		g.RecordFailure(ctx, ip, "")
	}
	status, reached := serve(IPPrecheckHandler(g), ip)
	if status != http.StatusTooManyRequests {
		t.Fatalf("被锁 IP 应 429,got %d", status)
	}
	if reached {
		t.Fatal("被锁 IP 不该进入 handler")
	}
}

// TestMiddlewareAllowsCleanIP 未被锁的 IP → 放行到 handler。
func TestMiddlewareAllowsCleanIP(t *testing.T) {
	g := newG(newFake(), hardLockPolicy())
	status, reached := serve(IPPrecheckHandler(g), ip)
	if status != http.StatusOK || !reached {
		t.Fatalf("干净 IP 应放行,got status=%d reached=%v", status, reached)
	}
}

// TestMiddlewareFailOpenNilGuard 未启用 loginguard(g==nil)→ fail-open 放行(可用性优先)。
func TestMiddlewareFailOpenNilGuard(t *testing.T) {
	status, reached := serve(IPPrecheckHandler(nil), ip)
	if status != http.StatusOK || !reached {
		t.Fatalf("nil guard 应 fail-open 放行,got status=%d reached=%v", status, reached)
	}
}

// TestMiddlewareWithOnBlocked 自定义被拦响应生效。
func TestMiddlewareWithOnBlocked(t *testing.T) {
	ctx := context.Background()
	g := newG(newFake(), hardLockPolicy())
	for i := 0; i < 3; i++ {
		g.RecordFailure(ctx, ip, "")
	}
	h := IPPrecheckHandler(g, WithOnBlocked(func(c *gin.Context, _ Decision) {
		c.JSON(http.StatusTeapot, gin.H{"code": "locked"})
	}))
	status, reached := serve(h, ip)
	if status != http.StatusTeapot {
		t.Fatalf("自定义响应应生效(418),got %d", status)
	}
	if reached {
		t.Fatal("被拦仍不该进 handler")
	}
}
