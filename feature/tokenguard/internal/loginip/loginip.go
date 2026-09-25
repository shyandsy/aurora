// Package loginip 把已签发的 token(按 jti)绑定到登录时的客户端 IP(tokenguard 内部实现)。
// 每次操作校验「当前请求 IP == 登录 IP」,不符即拒 —— 防 token 泄漏后被异地重放。
//
// key = tokenip:<jti>,值 = 登录时客户端 IP,TTL 与 token 有效期一致(到期自动清)。
//
// 写路径(Bind/Unbind)一律返回 error,绝不静默 no-op:Bind 是「建立 IP 绑定」这半防护,静默跳过
// (拿不到 IP、后端 nil、写失败)= 调用方以为 token 受保护、实际没有,而绑 IP 的 token 因「未绑定」
// 走 Match 的 fail-close,表现为「登录成功却每请求 401」的静默故障。
package loginip

import (
	"context"
	"errors"
	"time"
)

const keyPrefix = "tokenip:"

var (
	// ErrBackendUnavailable 后端不可用(nil)。读写失败则原样返回底层 error。
	ErrBackendUnavailable = errors.New("loginip: backend unavailable")
	// ErrInvalidArgs 建立绑定参数非法(jti/ip 空或 ttl<=0)。
	ErrInvalidArgs = errors.New("loginip: invalid binding args (empty jti/ip or non-positive ttl)")
)

func key(jti string) string { return keyPrefix + jti }

// Getter / Setter / Deleter 是本包所需的最小接口(tokenguard.Redis 天然满足)。
type Getter interface {
	Get(ctx context.Context, key string) (string, error)
}
type Setter interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
}
type Deleter interface {
	Delete(ctx context.Context, keys ...string) (int64, error)
}

// Bind 记录某 token(jti)绑定的登录 IP,ttl 与 token 有效期一致。参数非法返回 ErrInvalidArgs,
// 写入失败原样返回底层 error —— 调用方必须处理(建立不了保护就别发这枚 token)。
func Bind(ctx context.Context, r Setter, jti, ip string, ttl time.Duration) error {
	if r == nil {
		return ErrBackendUnavailable
	}
	if jti == "" || ip == "" || ttl <= 0 {
		return ErrInvalidArgs
	}
	return r.Set(ctx, key(jti), ip, ttl)
}

// Unbind 删除某 token(jti)的 IP 绑定(撤销会话的一半)。后端 nil 返回 ErrBackendUnavailable;
// jti 空视为无事可删(nil);删除失败原样返回。
func Unbind(ctx context.Context, r Deleter, jti string) error {
	if r == nil {
		return ErrBackendUnavailable
	}
	if jti == "" {
		return nil
	}
	_, err := r.Delete(ctx, key(jti))
	return err
}

// Match 校验当前 IP 是否与该 token 绑定的登录 IP 一致。
// (ok,err):err!=nil = 后端不可用(fail-close 拒);ok=false = 未绑定或不符(拒);ok=true = 一致(放行)。
func Match(ctx context.Context, r Getter, jti, ip string) (bool, error) {
	if r == nil {
		return false, ErrBackendUnavailable
	}
	v, err := r.Get(ctx, key(jti))
	if err != nil {
		return false, err
	}
	return v != "" && v == ip, nil
}
