package role_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/datalayer"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	"github.com/shyandsy/aurora/modules/user/service/rolefeaturepublisher"
)

// RoleFeatureService 角色功能关联服务接口
type RoleFeatureService interface {
	GetRoleFeatures(ctx *contracts.RequestContext, roleID int64) ([]dto.RoleFeature, bizerr.BizError)
	GetRoleFeature(ctx *contracts.RequestContext, id int64) (*dto.RoleFeature, bizerr.BizError)
	CreateRoleFeature(ctx *contracts.RequestContext, req dto.CreateRoleFeatureReq) (*dto.RoleFeature, bizerr.BizError)
	DeleteRoleFeature(ctx *contracts.RequestContext, id int64) bizerr.BizError
	DeleteRoleFeatureByRoleAndFeature(ctx *contracts.RequestContext, roleID, featureID int64) bizerr.BizError
}

// roleFeatureService 角色功能关联服务实现
type roleFeatureService struct {
	DL        datalayer.RoleFeatureDatalayer `inject:""`
	RoleDL    datalayer.RoleDatalayer        `inject:""`
	FeatureDL datalayer.FeatureDatalayer     `inject:""`
	// Publisher:role↔feature 关联变更后,把受影响 role 的最新展开 feature 写穿进共享读模型。best-effort。
	Publisher rolefeaturepublisher.Publisher `inject:""`
}

// NewRoleFeatureService 创建角色功能关联服务
func NewRoleFeatureService(app contracts.App) RoleFeatureService {
	s := &roleFeatureService{}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve RoleFeatureService: %w", err))
	}
	return s
}
