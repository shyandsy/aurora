// Package tokenguard 是 aurora 的「token 会话有效性」内核:撤销(jti 黑名单)+ IP 绑定(防盗用重放)+
// token 自描述作用域约定。配合 aurora/feature/jwt 使用:jwt 管「密码学上有效吗」,tokenguard 管
// 「这枚有效 token 在会话层面还该被接受吗」(登出了没、被踢了没、异地重放没)。
//
// 对外只暴露:
//   - Guard 接口(有状态、redis 支撑的会话操作)——经 aurora feature 注入,见 feature.go;
//   - SessionTags / ScopeOf / PublicFeatures / DefaultScope(纯 scope 约定助手,无需构造);
//   - NewTokenGuardFeature(默认 feature)/ NewGuardWithRedis(薄服务自带 redis 构造)。
//
// 实现细节(jti 黑名单、IP 绑定、scope 标记)全在 internal/,外部无法 import,可随内部重构而不破坏契约。
package tokenguard

import (
	"context"
	"errors"
	"time"

	auroraFeature "github.com/shyandsy/aurora/feature"

	"github.com/shyandsy/aurora/feature/tokenguard/internal/denylist"
	"github.com/shyandsy/aurora/feature/tokenguard/internal/loginip"
	"github.com/shyandsy/aurora/feature/tokenguard/internal/scope"
)

// 会话校验失败原因。调用方据此映射响应文案,但语义统一:一律 fail-close 拒绝(401),绝不放行。
var (
	// ErrRevoked token 的 jti 已在撤销黑名单。
	ErrRevoked = errors.New("token has been revoked")
	// ErrIPMismatch 当前请求 IP 与登录 IP 不符(或该 token 未绑定 IP → fail-close 视作不符)。
	ErrIPMismatch = errors.New("operation IP does not match login IP")
	// ErrBackendUnavailable 校验所需的 Redis 后端不可用(fail-close)。
	ErrBackendUnavailable = errors.New("session verification backend unavailable")
)

// DefaultScope 无 scope 标记的历史 token 归入的作用域(web:绑 IP、可达管理面)。
const DefaultScope = scope.DefaultScope

// Redis 是 Guard 所需的最小 Redis 能力(仅 Get/Set/Delete)。aurora 的 RedisService 天然满足;
// 无 aurora DI 的薄服务(如 BFF)用一个几方法的 go-redis 适配器即可,拿到同一个完整 Guard。
type Redis interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) (int64, error)
}

// Guard 是「token 会话级有效性」的唯一对外契约。经 aurora feature 注入(见 NewTokenGuardFeature):
//
//	type userService struct {
//	    TokenGuard tokenguard.Guard `inject:""`
//	}
type Guard interface {
	// VerifySession 校验**浏览器直连面**的会话有效性:jti 未被撤销 + (按 token 的 sess:noip 策略)
	// 当前 IP == 登录 IP。任一 Redis 读失败一律 fail-close。claims 提供 jti/features,调用方无需再拆。
	VerifySession(ctx context.Context, claims *auroraFeature.Claims, clientIP string) error

	// VerifySessionProxied 校验**被代理/内部面**的会话有效性:只查撤销,不做 IP 绑定 —— 那时请求 IP
	// 是代理容器 IP,做绑定必然误杀。故不收 clientIP。
	VerifySessionProxied(ctx context.Context, claims *auroraFeature.Claims) error

	// BindLoginIP 在签发时把 token(jti)绑定到登录 IP(ttl = token 剩余寿命)。返回 error 必须处理:
	// 建立不了绑定就别发这枚 token(否则 web token 该绑没绑 → 登录成功却每请求 401)。
	BindLoginIP(ctx context.Context, jti, ip string, ttl time.Duration) error

	// RevokeClaims 彻底失效 claims 对应的 token(删 IP 绑定 + 入黑名单)。黑名单 TTL 从 claims.ExpiresAt
	// 精确推(自然过期即清,轮换热路径不留垃圾);无到期信息回落 denyTTL;已过期只删 IP 绑定。
	// 返回 error:任一半没成功都上报(errors.Join),调用方别假装登出成功。
	RevokeClaims(ctx context.Context, claims *auroraFeature.Claims) error

	// Revoke 只有 jti、拿不到到期信息时用(如会话表只存了 cur_access_jti / cur_refresh_jti)。
	// 黑名单 TTL 用构造时配的 denyTTL。撤销罕见,轻微过存无碍。
	Revoke(ctx context.Context, jti string) error
}

// SessionTags 按作用域策略生成签发时要写进 JWT features 的内部标记:scope:<name>,bindIP=false 时追加
// sess:noip。签发方:jwt.GenerateToken(uid, email, append(业务features, tokenguard.SessionTags(scope,bind)...))。
func SessionTags(scopeName string, bindIP bool) []string {
	tags := []string{scope.Tag(scopeName)}
	if !bindIP {
		tags = append(tags, scope.NoIPTag())
	}
	return tags
}

// ScopeOf 从 features 解析 token 的作用域名(供路由网关按 scope 施策);无标记返回 DefaultScope。
func ScopeOf(features []string) string { return scope.Of(features) }

// PublicFeatures 过滤掉内部标记(scope:/sess:noip),返回可安全回给客户端的 features。
func PublicFeatures(features []string) []string {
	out := make([]string, 0, len(features))
	for _, f := range features {
		if !scope.IsInternalTag(f) {
			out = append(out, f)
		}
	}
	return out
}

type guard struct {
	redis   Redis
	denyTTL time.Duration
}

// NewGuardWithRedis 用调用方自带的 Redis 构造 Guard —— 供无 aurora DI 的薄服务(BFF)与单测使用;
// 完整 aurora 服务走 NewTokenGuardFeature(自动注入)。denyTTL/redis 非法即 panic(见 feature.go 说明)。
func NewGuardWithRedis(redis Redis, denyTTL time.Duration) Guard {
	if denyTTL <= 0 {
		panic("tokenguard: denyTTL 必须 > 0(应 >= 最长 token/refresh 寿命),否则撤销会静默失效")
	}
	if redis == nil {
		panic("tokenguard: redis 不能为 nil —— 鉴权全面 fail-close,拒绝带病启动")
	}
	return &guard{redis: redis, denyTTL: denyTTL}
}

func (g *guard) VerifySessionProxied(ctx context.Context, claims *auroraFeature.Claims) error {
	if g.redis == nil || claims == nil {
		return ErrBackendUnavailable
	}
	revoked, err := denylist.IsRevoked(ctx, g.redis, claims.ID)
	if err != nil {
		return ErrBackendUnavailable // fail-close
	}
	if revoked {
		return ErrRevoked
	}
	return nil
}

func (g *guard) VerifySession(ctx context.Context, claims *auroraFeature.Claims, clientIP string) error {
	if err := g.VerifySessionProxied(ctx, claims); err != nil {
		return err // 复用撤销校验(含 nil / fail-close)
	}
	// 是否校 IP 完全依 token 自带 sess:noip 标记,此处零客户端类型判断。
	if scope.SkipIPBinding(claims.Features) {
		return nil
	}
	ok, err := loginip.Match(ctx, g.redis, claims.ID, clientIP)
	if err != nil {
		return ErrBackendUnavailable // fail-close
	}
	if !ok {
		return ErrIPMismatch
	}
	return nil
}

func (g *guard) BindLoginIP(ctx context.Context, jti, ip string, ttl time.Duration) error {
	return loginip.Bind(ctx, g.redis, jti, ip, ttl)
}

func (g *guard) RevokeClaims(ctx context.Context, claims *auroraFeature.Claims) error {
	if claims == nil {
		return nil // 无 token 可撤,不是失败
	}
	if claims.ExpiresAt != nil {
		remaining := time.Until(claims.ExpiresAt.Time)
		if remaining <= 0 {
			return loginip.Unbind(ctx, g.redis, claims.ID) // 已过期:黑名单半无必要,只删 IP 绑定
		}
		return g.revoke(ctx, claims.ID, remaining) // 精确 TTL:自然过期即清
	}
	return g.revoke(ctx, claims.ID, g.denyTTL) // 无到期信息 → denyTTL 兜底
}

func (g *guard) Revoke(ctx context.Context, jti string) error {
	return g.revoke(ctx, jti, g.denyTTL)
}

// revoke 彻底失效一枚 token:删 IP 绑定(web 靠它)+ 入 jti 黑名单(app 靠它)。两半都尽力做完,
// errors.Join 合并上报——半成功(如只删了 IP 绑定、黑名单没写)对不绑 IP 的 App token 就是没真吊销,
// 调用方必须知道,不能静默吞。
func (g *guard) revoke(ctx context.Context, jti string, ttl time.Duration) error {
	if g.redis == nil {
		return ErrBackendUnavailable
	}
	if jti == "" {
		return nil // 无 jti 可撤,不是失败
	}
	return errors.Join(
		loginip.Unbind(ctx, g.redis, jti),
		denylist.RevokeWithTTL(ctx, g.redis, jti, ttl),
	)
}
