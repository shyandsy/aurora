package role_feature

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/datalayer"
	"github.com/shyandsy/aurora/sample/model/dto"
)

// RoleFeatureService 角色功能关联服务接口
type RoleFeatureService interface {
	GetRoleFeatures(ctx *contracts.RequestContext, roleID int64) ([]dto.RoleFeature, bizerr.BizError)
	GetRoleFeature(ctx *contracts.RequestContext, id int64) (*dto.RoleFeature, bizerr.BizError)
	CreateRoleFeature(ctx *contracts.RequestContext, req dto.CreateRoleFeatureReq) (*dto.RoleFeature, bizerr.BizError)
	DeleteRoleFeature(ctx *contracts.RequestContext, id int64) bizerr.BizError
}

// roleFeatureService 角色功能关联服务实现
type roleFeatureService struct {
	DL        datalayer.RoleFeatureDatalayer `inject:""`
	RoleDL    datalayer.RoleDatalayer        `inject:""`
	FeatureDL datalayer.FeatureDatalayer     `inject:""`
}

// NewRoleFeatureService 创建角色功能关联服务
func NewRoleFeatureService(app contracts.App) RoleFeatureService {
	s := &roleFeatureService{}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve RoleFeatureService: %w", err))
	}
	return s
}
