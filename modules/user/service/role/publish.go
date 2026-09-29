package role

import (
	"context"

	"github.com/shyandsy/aurora/logger"
)

// publishRoleReadModel 把某 role 的**展开** feature 写穿进共享读模型(role/role_feature 变更成功后调)。
// best-effort:展开或写入失败只记日志、不返回错误(本批发号仍 feature-in-token,读模型当前无消费者)。
// 必须在 DB 写成功之后调(先 DB 后 cache)。
func (s *roleService) publishRoleReadModel(ctx context.Context, roleID int64, roleName string) {
	names, err := s.FeatureDL.GetExpandedNamesByRoleID(ctx, roleID)
	if err != nil {
		logger.Errorf("[rolefeature] 展开 role id=%d name=%q 失败,跳过发布: %v", roleID, roleName, err)
		return
	}
	if err := s.Publisher.PublishRole(ctx, roleName, names); err != nil {
		logger.Errorf("[rolefeature] 发布 role name=%q 到读模型失败: %v", roleName, err)
	}
}

// removeRoleReadModel 从共享读模型删除某 role 的键(role 删除成功后调)。best-effort。
func (s *roleService) removeRoleReadModel(ctx context.Context, roleName string) {
	if err := s.Publisher.RemoveRole(ctx, roleName); err != nil {
		logger.Errorf("[rolefeature] 从读模型删除 role name=%q 失败: %v", roleName, err)
	}
}
