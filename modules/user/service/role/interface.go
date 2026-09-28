package role

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/datalayer"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	"github.com/shyandsy/aurora/modules/user/service/rolefeaturepublisher"
)

// RoleService 角色服务接口
type RoleService interface {
	GetRoles(ctx *contracts.RequestContext) ([]dto.Role, bizerr.BizError)
	GetRole(ctx *contracts.RequestContext, id int64) (*dto.Role, bizerr.BizError)
	CreateRole(ctx *contracts.RequestContext, req dto.CreateRoleReq) (*dto.Role, bizerr.BizError)
	UpdateRole(ctx *contracts.RequestContext, id int64, req dto.UpdateRoleReq) (*dto.Role, bizerr.BizError)
	DeleteRole(ctx *contracts.RequestContext, id int64) bizerr.BizError
}

// roleService 角色服务实现
type roleService struct {
	DL datalayer.RoleDatalayer `inject:""`
	// FeatureDL 用于把 role 展开成 feature name(签发时展开的同一函数),供发布读模型。
	FeatureDL datalayer.FeatureDatalayer `inject:""`
	// Publisher 把 role→展开 feature 写穿进共享 Redis 读模型(双模中间件按 role 判权用)。
	// 本批发号仍 feature-in-token、读模型当前无消费者,故发布 best-effort(失败只记日志,不阻断 CRUD)。
	Publisher rolefeaturepublisher.Publisher `inject:""`
}

// NewRoleService 创建角色服务
func NewRoleService(app contracts.App) RoleService {
	s := &roleService{}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve RoleService: %w", err))
	}
	return s
}
