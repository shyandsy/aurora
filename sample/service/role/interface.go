package role

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/datalayer"
	"github.com/shyandsy/aurora/sample/model/dto"
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
}

// NewRoleService 创建角色服务
func NewRoleService(app contracts.App) RoleService {
	s := &roleService{}
	if err := app.Resolve(s); err != nil {
		panic(fmt.Errorf("failed to resolve RoleService: %w", err))
	}
	return s
}
