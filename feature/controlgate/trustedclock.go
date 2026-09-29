package controlgate

import (
	"context"
	"sync"
	"time"

	"github.com/shyandsy/aurora/logger"
)

// timeSourceTimeout 单个 timeSource 取时的超时(时钟是旁路,绝不为它久等)。
const timeSourceTimeout = 5 * time.Second

// timeSource 是 control 之外的一个「可信当前时间」权威来源。实现**必须自证**(签名验签 / 证书 pin),
// 只在验证通过时返回时间;拿不到 / 验不过返回 error(调用方跳过)。
//
// 现在只留接口 + 内部 NTS 源实现它。裸 NTP 不满足本接口(甲方是 root + 控出口,能伪造未签名时间)——
// 实现必须是可验签的(NTS/Roughtime)。
type timeSource interface {
	// Name 源名(日志 / 观测用)。
	Name() string
	// Available 本源当前是否有可试的目标(如 NTS:是否已被 control 下发过非空服务器名单)。
	// air-gap fail-closed 守卫只在**存在 Available 源**时启用:名单为空(老 control 没下发 /
	// 首个裁决之前)不能开 air-gap,否则会把拿不到源的合法机器误锁。
	Available() bool
	// Now 返回该源的可信 unix 秒;拿不到 / 验不过返回 error。
	Now(ctx context.Context) (int64, error)
}

// hasAvailableSource 是否存在「当前可试」的时间源(见 timeSource.Available)。
func (g *gate) hasAvailableSource() bool {
	for _, s := range g.sources {
		if s.Available() {
			return true
		}
	}
	return false
}

// trustedClock 收口「可信当前时间从哪来 + 信不信得过」。降级链:
//
//	① 权威样本(control 裁决 now / NTS 签名时间)→ 锚点 + 本地单调往前数(运行中改墙钟无效)
//	② 跨重启:用签名裁决的 now 当「不可回拨下界」floor —— 它在 ed25519 签名内,甲方改不了、只能删
//	③ 本会话拿不到任何新鲜样本、又超启动宽限 → now() 报 fresh=false → 上层 fail-closed(判被隔离)
//
// 与「授权」解耦:本时钟只回答「几点 + 信不信」,不碰授权判定;授权只认 control 签名裁决。
type trustedClock struct {
	started time.Time // 门禁启动的单调时刻(算启动宽限)

	mu       sync.RWMutex
	anchor   int64     // 最近权威样本的 unix
	anchorAt time.Time // 观测该样本时的本地单调时刻
	fresh    bool      // 本会话是否拿到过至少一个新鲜权威样本(control 或 NTS)
	floor    int64     // 不可回拨下界(签名裁决 now;启动时从持久化 verdict 载入)
}

func newTrustedClock() *trustedClock { return &trustedClock{started: time.Now()} }

// observe 记录一个新鲜权威时间样本(control 裁决 now,或 NTS 签名时间)。
func (c *trustedClock) observe(unix int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.anchor = unix
	c.anchorAt = time.Now()
	c.fresh = true
	if unix > c.floor {
		c.floor = unix
	}
}

// seedFloor 启动时用持久化签名裁决的 now 设不可回拨下界(**不算新鲜样本**:它来自过去会话)。
func (c *trustedClock) seedFloor(unix int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if unix > c.floor {
		c.floor = unix
	}
}

// now 返回当前可信时间 + 是否「新鲜」。
//   - fresh=true:有本会话权威锚点 → anchor + 单调流逝(不低于 floor),可信;
//   - fresh=false:本会话没连上任何权威源 → 退化墙钟,但不低于 floor(签名下界,防回拨),并告知上层不可信。
func (c *trustedClock) now() (int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var n int64
	if c.fresh {
		n = c.anchor + int64(time.Since(c.anchorAt).Seconds())
	} else {
		n = time.Now().Unix()
	}
	if n < c.floor {
		n = c.floor // 不可回拨下界(签名 now)
	}
	return n, c.fresh
}

// sinceStart 门禁启动至今(算启动宽限)。
func (c *trustedClock) sinceStart() time.Duration { return time.Since(c.started) }

// pollForTime 依次问各 timeSource 要一次可信时间,拿到第一个成功的就 observe 并返回 true。
// control 续约失败时调用,用签名时间源续住时钟(拿不到授权,但能保 fresh)。整体有超时;逐源失败静默跳过。
func (g *gate) pollForTime(ctx context.Context) bool {
	for _, src := range g.sources {
		sctx, cancel := context.WithTimeout(ctx, timeSourceTimeout)
		unix, err := src.Now(sctx)
		cancel()
		if err == nil && unix > 0 {
			g.clock.observe(unix)
			return true
		}
		if g.debug {
			logger.Debugf("controlgate[%s]: 时间源 %s 取时失败: %v", g.name, src.Name(), err)
		}
	}
	return false
}
