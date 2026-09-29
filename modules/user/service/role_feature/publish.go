package role_feature

import (
	"context"

	"github.com/shyandsy/aurora/logger"
)

// republishRole 在 role↔feature 关联变更成功后,重算受影响 role 的展开 feature 并写穿读模型。
// best-effort:role 查不到 / 展开失败 / 写入失败均只记日志,不阻断 CRUD。必须在 DB 写成功之后调。
func (s *roleFeatureService) republishRole(ctx context.Context, roleID int64) {
	role, err := s.RoleDL.GetByID(ctx, roleID)
	if err != nil {
		logger.Errorf("[rolefeature] 读 role id=%d 失败,跳过发布: %v", roleID, err)
		return
	}
	if role == nil {
		return // role 已不存在(并发删除等)→ 无需写穿
	}
	names, err := s.FeatureDL.GetExpandedNamesByRoleID(ctx, roleID)
	if err != nil {
		logger.Errorf("[rolefeature] 展开 role id=%d name=%q 失败,跳过发布: %v", roleID, role.Name, err)
		return
	}
	if err := s.Publisher.PublishRole(ctx, role.Name, names); err != nil {
		logger.Errorf("[rolefeature] 发布 role name=%q 到读模型失败: %v", role.Name, err)
	}
}
