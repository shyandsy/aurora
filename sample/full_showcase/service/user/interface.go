package user

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/sample/full_showcase/datalayer"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	commonModel "github.com/shyandsy/aurora/sample/full_showcase/common/model"
)

// UserService is the user service interface.
type UserService interface {
	Login(ctx *contracts.RequestContext, req dto.LoginReq) (*dto.LoginResp, bizerr.BizError)
	GetUsers(ctx *contracts.RequestContext, req commonModel.PagingReq) (*commonModel.PagingResponse, bizerr.BizError)
	GetUser(ctx *contracts.RequestContext, id int64) (*dto.User, bizerr.BizError)
	CreateUser(ctx *contracts.RequestContext, req dto.CreateUserReq) (*dto.User, bizerr.BizError)
	UpdateUser(ctx *contracts.RequestContext, id int64, req dto.UpdateUserReq) (*dto.User, bizerr.BizError)
	DeleteUser(ctx *contracts.RequestContext, id int64) bizerr.BizError
}

// userService is the user service implementation.
type userService struct {
	DL        datalayer.UserDatalayer    `inject:""`
	RoleDL    datalayer.RoleDatalayer    `inject:""`
	FeatureDL datalayer.FeatureDatalayer `inject:""`
	JWT       auroraFeature.JWTService   `inject:""`
}

// NewUserService creates the user service.
func NewUserService(app contracts.App) UserService {
	c := &userService{}
	if err := app.Resolve(c); err != nil {
		panic(fmt.Errorf("failed to resolve UserService: %w", err))
	}
	return c
}
