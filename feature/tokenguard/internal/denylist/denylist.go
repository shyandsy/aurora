// Package denylist 是按 jti 的 token 撤销黑名单(tokenguard 内部实现)。
//
// 与 loginip(按 IP 绑定)互补:loginip 只对"绑了登录 IP"的 web token 有效,删绑定即失效;但不绑 IP
// 的 token(如移动 App)无绑定可删 —— 撤销就靠这个 jti 黑名单。命中即拒(fail-close)。
//
// key = tokendeny:<jti>,值 "1"。TTL 必须覆盖被撤 token 的完整剩余寿命,否则条目先于 token 过期 →
// 被撤 token(尤其不绑 IP 的)会在 TTL 到点后"复活"。故本包不提供默认 TTL,由调用方(tokenguard.Guard)
// 按 token 剩余寿命 / denyTTL 显式传。
package denylist

import (
	"context"
	"errors"
	"time"
)

const keyPrefix = "tokendeny:"

var (
	// ErrBackendUnavailable 后端不可用(nil)。读写失败则原样返回底层 error。
	ErrBackendUnavailable = errors.New("denylist: backend unavailable")
	// ErrInvalidArgs 入黑名单参数非法(jti 空或 ttl<=0)。撤销是安全写,建立不了必须上报,绝不静默跳过。
	ErrInvalidArgs = errors.New("denylist: invalid args (empty jti or non-positive ttl)")
)

func key(jti string) string { return keyPrefix + jti }

// Getter / Setter 是本包所需的最小读/写接口(tokenguard.Redis 天然满足)。
type Getter interface {
	Get(ctx context.Context, key string) (string, error)
}
type Setter interface {
	Set(ctx context.Context, key string, value any, ttl time.Duration) error
}

// RevokeWithTTL 把某 token(jti)加入黑名单,TTL 由调用方按剩余寿命显式给出。参数非法返回 ErrInvalidArgs,
// 写入失败原样返回底层 error —— 绝不静默 no-op。
func RevokeWithTTL(ctx context.Context, r Setter, jti string, ttl time.Duration) error {
	if r == nil {
		return ErrBackendUnavailable
	}
	if jti == "" || ttl <= 0 {
		return ErrInvalidArgs
	}
	return r.Set(ctx, key(jti), "1", ttl)
}

// IsRevoked 查某 token(jti)是否已被撤销。err!=nil = 后端不可用(调用方 fail-close 拒)。
func IsRevoked(ctx context.Context, r Getter, jti string) (bool, error) {
	if r == nil {
		return false, ErrBackendUnavailable
	}
	v, err := r.Get(ctx, key(jti))
	if err != nil {
		return false, err
	}
	return v != "", nil
}
