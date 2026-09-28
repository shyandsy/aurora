// Package rolefeaturepublisher 是「role→feature 读模型」的**写侧 DI 服务**:把某 role 的
// 展开 feature 写穿进共享 Redis 读模型(common/middleware/rolefeature),供各调用方中间件按
// role 展开判权而无需查签发方的库。
//
// 与读侧的关系(写读一处收口):
//   - 键约定 / 序列化 / fail-close 全部收口在 common/middleware/rolefeature(读原语 + 写原语);
//   - 本包只是这些原语的**可注入包装**(interface + impl + New(app)),让业务 service 能 inject
//     一个 Publisher、在 role / role_feature CRUD 成功后 best-effort 发布,启动时全量重建。
//
// 中性:不含任何业务名 / 域名;展开 feature 由调用方(各服务用自己的 datalayer)算好后传入,
// 本包不依赖任何服务的 datalayer,可被 admin / user 等任意认证服务共用。
//
// 写穿纪律:调用方**必须在 DB 写成功之后**才调 PublishRole/RemoveRole(先 DB 后 cache,
// 绝不 DB 失败却写 cache = 凭空发权限)。发布失败对当前批次(feature-in-token)无消费者,
// 调用方按 best-effort 记日志、不阻断 CRUD 即可。
package rolefeaturepublisher

import (
	"context"
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"

	"github.com/shyandsy/aurora/middleware/rolefeature"
)

// Publisher 是 role→feature 读模型写侧的可注入接口。
//
// 展开语义留给调用方:expandedFeatures 必须是该 role 的**完整展开** feature name 列表
// (调用方用自己的 datalayer.GetExpandedNamesByRoleID 算),本包整键覆盖写入,不做增量。
type Publisher interface {
	// PublishRole 整键覆盖某 role 的全量展开 feature(DB 写成功后调)。
	// expandedFeatures 为空也会写入空集(而非删键),与「键缺失」区分。
	PublishRole(ctx context.Context, roleName string, expandedFeatures []string) error
	// RemoveRole 删除某 role 的读模型键(role 被删后调)。删后该 role 解析将 miss→fail-close。
	RemoveRole(ctx context.Context, roleName string) error
	// RebuildAll 全量重建读模型(启动时调):all 是 roleName→展开 feature 的全量映射。
	RebuildAll(ctx context.Context, all map[string][]string) error
}

// publisher 是 Publisher 的默认实现,只持有共享 Redis;所有键约定 / 序列化委托给 rolefeature 读侧。
type publisher struct {
	Redis auroraFeature.RedisService `inject:""`
}

// NewPublisher 构造 Publisher。返回接口(可空)以满足 DI ProvideAs 的可空要求。
func NewPublisher(app contracts.App) Publisher {
	s := &publisher{}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve rolefeaturepublisher.Publisher: %w", err))
	}
	return s
}

func (s *publisher) PublishRole(ctx context.Context, roleName string, expandedFeatures []string) error {
	return rolefeature.PublishRole(ctx, s.Redis, roleName, expandedFeatures)
}

func (s *publisher) RemoveRole(ctx context.Context, roleName string) error {
	return rolefeature.InvalidateRole(ctx, s.Redis, roleName)
}

func (s *publisher) RebuildAll(ctx context.Context, all map[string][]string) error {
	return rolefeature.RebuildAll(ctx, s.Redis, all)
}
