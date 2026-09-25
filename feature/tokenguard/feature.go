package tokenguard

import (
	"fmt"
	"time"

	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
)

// tokenGuardFeature 是把 Guard 装进 aurora DI 的 feature:注册后服务经 `inject:""` 拿到 Guard,
// 无需在 providers.go 手动 ProvideAs。
//
// opt-in feature(不在 bootstrap 默认集):鉴权服务显式 AddFeature。不像 jwt/redis 那么普适——
// 纯 worker / 只用 API-key 的服务不需要会话守卫,不该被强加。
type tokenGuardFeature struct {
	Redis auroraFeature.RedisService `inject:""`

	denyTTL time.Duration
}

// NewTokenGuardFeature 构造 token 会话有效性 feature。须在 redis feature 之后注册。
//
// denyTTL = 黑名单兜底 TTL(仅 Revoke(jti) 无到期信息时用;RevokeClaims 永远按 claims.ExpiresAt 精确推),
// **必须 >= 本服务最长 token(通常 refresh)寿命**,否则不绑 IP 的 token(黑名单是其唯一吊销手段)会在
// 到点后"复活"。由调用方按自己的 token 策略显式传——不写死从 jwt 读,是因为服务的最长 token 寿命未必
// 等于 jwt refresh 配置(可能自签更长寿 token)。通常就传 jwt 的 refresh 寿命:
//
//	var jc config.JWTConfig
//	_ = config.ResolveConfig(&jc)
//	app.AddFeature(tokenguard.NewTokenGuardFeature(jc.RefreshExpireOrDefault()))
func NewTokenGuardFeature(denyTTL time.Duration) contracts.Features {
	return &tokenGuardFeature{denyTTL: denyTTL}
}

func (f *tokenGuardFeature) Name() string { return "tokenguard" }

func (f *tokenGuardFeature) Setup(app contracts.App) error {
	// 任何失败一律 return error → aurora AddFeature log.Fatalf 启动即退,绝不静默降级成全站 401。
	if f.denyTTL <= 0 {
		return fmt.Errorf("tokenguard: denyTTL 必须 > 0(应 >= 本服务最长 token/refresh 寿命),否则撤销会静默失效")
	}
	if err := app.Resolve(f); err != nil {
		return fmt.Errorf("tokenguard: 解析 RedisService 失败(鉴权将全面 fail-close): %w", err)
	}
	if f.Redis == nil {
		return fmt.Errorf("tokenguard: RedisService 未注入 —— 须在 tokenguard 之前注册 redis feature")
	}

	app.ProvideAs(NewGuardWithRedis(f.Redis, f.denyTTL), (*Guard)(nil))
	return nil
}

func (f *tokenGuardFeature) Close() error { return nil }
