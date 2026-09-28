// Package rolefeature 是 role→feature 的**共享读模型**收口(键约定 + 读/写穿/重建),
// 让 token 只存 roleID、中间件按 role 展开 feature 判权,而**无需在中间件里查 db**。
//
// 为什么要它:一 API 一 feature 后,把 feature 摊平进 token 会让 token 无限膨胀;改存 roleID 更紧凑,
// 但中间件就得能把 role 展开成 feature。而共享中间件(各调用方服务)读不到签发方的库
// (微服务隔离),所以走 Redis 读模型:**认证服务(数据主人)写、中间件读同一个 Redis,miss = fail-close**。
// 这与 tokenip / tokendeny 完全同一套路(签发方写、中间件读、fail-close),不是新架构。
//
// 契约:
//   - 键:rolefeature:role:<roleName> → JSON []featureName;**整键覆盖,不做增量**(防并发丢更新);
//   - 写:必须**先 DB 后 cache**(DB 成功才写;绝不 DB 失败却写 cache = 凭空发权限);
//   - 读:任一 role 键缺失(未初始化/被驱逐)或后端出错 → fail-close;
//   - 冷启动:由签发方写穿 + 启动全量 RebuildAll 保证键常热。
package rolefeature

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	auroraFeature "github.com/shyandsy/aurora/feature"
)

const keyPrefix = "rolefeature:role:"

// TTL 读模型键的存活时长。0 = 不过期(读模型靠写穿 + 启动重建维持,不靠 TTL 老化)。
// 如担心 Redis 驱逐,请在写侧加周期性 RebuildAll,而非缩短 TTL(缩短会带来到期后 miss→fail-close 的锁死窗口)。
const TTL time.Duration = 0

// ErrBackendUnavailable 读模型不可用:后端出错,或 role 键缺失(未初始化/被驱逐)。一律 fail-close。
var ErrBackendUnavailable = errors.New("rolefeature read-model unavailable")

// key 以角色名作键(与 token 里 Claims.Roles 存的名字一致)。名字是 role 的唯一自然键。
func key(roleName string) string { return keyPrefix + roleName }

// Store 是解析所需的**最小只读**接口(aurora RedisService 天然满足;单测可用内存假实现)。
type Store interface {
	Get(ctx context.Context, key string) (string, error)
}

// ResolveFeatures 把一组 roleName 展开成 feature 并集(供中间件按 requiredFeature 判权)。
//
// fail-close:任一 role 键读出错、或键缺失(空串,表示未初始化/被驱逐)→ 返回 ErrBackendUnavailable。
// 注意区分「键缺失」(Get 返回 "" → miss → 拒)与「角色确无 feature」(键值 "[]" → 合法空集,不拒),
// 故写侧对零 feature 的 role 也要写入 "[]" 而非不写。
func ResolveFeatures(ctx context.Context, store Store, roleNames []string) (map[string]struct{}, error) {
	if store == nil {
		return nil, ErrBackendUnavailable
	}
	out := make(map[string]struct{})
	for _, rn := range roleNames {
		raw, err := store.Get(ctx, key(rn))
		if err != nil {
			return nil, ErrBackendUnavailable // 后端错 → fail-close
		}
		if raw == "" {
			return nil, ErrBackendUnavailable // 键缺失 → fail-close(不可当作空权限放行)
		}
		var names []string
		if uerr := json.Unmarshal([]byte(raw), &names); uerr != nil {
			return nil, ErrBackendUnavailable // 脏数据 → fail-close
		}
		for _, n := range names {
			out[n] = struct{}{}
		}
	}
	return out, nil
}

// PublishRole 整键覆盖某 role 的全量 feature(写侧,签发方 admin 改动后调;**须在 DB 写成功之后**)。
// features 为空也写入 "[]"(表示"该 role 无 feature",与"键缺失"区分)。
func PublishRole(ctx context.Context, redis auroraFeature.RedisService, roleName string, features []string) error {
	if redis == nil {
		return ErrBackendUnavailable
	}
	if features == nil {
		features = []string{}
	}
	b, err := json.Marshal(features)
	if err != nil {
		return err
	}
	return redis.Set(ctx, key(roleName), string(b), TTL)
}

// InvalidateRole 删除某 role 的键(role 被删时调)。删后该 role 的 token 解析将 miss→fail-close。
func InvalidateRole(ctx context.Context, redis auroraFeature.RedisService, roleName string) error {
	if redis == nil {
		return ErrBackendUnavailable
	}
	_, err := redis.Delete(ctx, key(roleName))
	return err
}

// RebuildAll 全量重建读模型(启动时调):把当前所有 role→feature 整键写入。
// 注:仅写入 all 中的 role;若需清理已删 role 的残留键,依赖 InvalidateRole 在删除时完成。
func RebuildAll(ctx context.Context, redis auroraFeature.RedisService, all map[string][]string) error {
	if redis == nil {
		return ErrBackendUnavailable
	}
	for name, feats := range all {
		if err := PublishRole(ctx, redis, name, feats); err != nil {
			return err
		}
	}
	return nil
}
