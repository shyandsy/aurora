package loginguard

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shyandsy/aurora/contracts"
)

// middleware.go —— loginguard 的「IP 预检」gin 中间件:handler(bcrypt)前,仅凭 IP 就把被锁 / 超小时
// 上限的请求挡在门外。它是 PrecheckIP 的**读锁一侧**;累加失败、触发上锁的**写锁**在登录流程的
// RecordFailure/RecordSuccess 里(见 Guard)。故单挂中间件不构成防爆破——两半都接上才成立。
//
// 账号维度不在此:中间件阶段还没解析 body、拿不到账号,账号预检(PrecheckAccount)在服务层做。

// MiddlewareOption 配置 IP 预检中间件的可选行为。
type MiddlewareOption func(*mwConfig)

type mwConfig struct {
	onBlocked func(c *gin.Context, d Decision)
}

// WithOnBlocked 覆盖「被拦」时的响应(默认 429 + 简短 JSON)。产品需要本地化文案 / 自有错误码信封时用它。
// 回调内应自行 c.Abort()——或用它写响应后由中间件统一 Abort(中间件在回调后必定 Abort,不再进 handler)。
func WithOnBlocked(fn func(c *gin.Context, d Decision)) MiddlewareOption {
	return func(cfg *mwConfig) { cfg.onBlocked = fn }
}

func defaultOnBlocked(c *gin.Context, _ Decision) {
	c.JSON(http.StatusTooManyRequests, gin.H{"message": "Too many login attempts, please try later"})
}

// IPPrecheckHandler 用一个 Guard 直接构造 IP 预检中间件(只依赖窄接口,便于测试与非 DI 场景)。
// g==nil(未启用 loginguard)或取不到客户端 IP → fail-open 放行,与 loginguard 整体「可用性优先」一致。
func IPPrecheckHandler(g Guard, opts ...MiddlewareOption) gin.HandlerFunc {
	cfg := mwConfig{onBlocked: defaultOnBlocked}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if g == nil || ip == "" { // fail-open
			c.Next()
			return
		}
		if dec := g.PrecheckIP(c.Request.Context(), ip); dec.Blocked {
			cfg.onBlocked(c, dec)
			c.Abort()
			return
		}
		c.Next()
	}
}

// IPPrecheckMiddleware 便利构造:从 aurora DI 解析 Guard(构造期一次,非每请求),再交给 IPPrecheckHandler。
// 未注册 loginguard feature 时 Find 失败、g 保持 nil → fail-open。须在 loginguard feature 注册之后构造。
func IPPrecheckMiddleware(app contracts.App, opts ...MiddlewareOption) gin.HandlerFunc {
	var g Guard
	_ = app.Find(&g)
	return IPPrecheckHandler(g, opts...)
}
