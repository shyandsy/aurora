package user

import (
	"fmt"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/feature/geoip"
	"github.com/shyandsy/aurora/feature/loginguard"
	"github.com/shyandsy/aurora/feature/tokenguard"
	"github.com/shyandsy/aurora/types"
	"github.com/shyandsy/aurora/modules/user/datalayer"
	"github.com/shyandsy/aurora/modules/user/model/dto"
)

// UserService 用户服务接口
type UserService interface {
	Login(ctx *contracts.RequestContext, req dto.LoginReq) (*dto.LoginResp, bizerr.BizError)
	Logout(ctx *contracts.RequestContext, req dto.LogoutReq) bizerr.BizError
	RefreshToken(ctx *contracts.RequestContext, req dto.RefreshReq) (*dto.RefreshResp, bizerr.BizError)
	GetUsers(ctx *contracts.RequestContext, req types.PagingReq) (*types.PagingResponse, bizerr.BizError)
	GetUser(ctx *contracts.RequestContext, id int64) (*dto.User, bizerr.BizError)
	CreateUser(ctx *contracts.RequestContext, req dto.CreateUserReq) (*dto.User, bizerr.BizError)
	UpdateUser(ctx *contracts.RequestContext, id int64, req dto.UpdateUserReq) (*dto.User, bizerr.BizError)
	DeleteUser(ctx *contracts.RequestContext, id int64) bizerr.BizError

	// 两步验证(TOTP,Google Authenticator)。setup/confirm/disable 需已登录;LoginTwoFactor 凭 pendingToken 免登录。
	SetupTotp(ctx *contracts.RequestContext, req dto.TotpSetupReq) (*dto.TotpSetupResp, bizerr.BizError)
	ConfirmTotp(ctx *contracts.RequestContext, req dto.TotpConfirmReq) (*dto.TotpConfirmResp, bizerr.BizError)
	DisableTotp(ctx *contracts.RequestContext, req dto.TotpDisableReq) bizerr.BizError
	LoginTwoFactor(ctx *contracts.RequestContext, req dto.Login2FAReq) (*dto.LoginResp, bizerr.BizError)

	// 登录会话/设备清单:列出我的会话、撤销单个、撤销其余(除当前)。
	ListSessions(ctx *contracts.RequestContext) ([]dto.SessionView, bizerr.BizError)
	RevokeSession(ctx *contracts.RequestContext, sessionID string) bizerr.BizError
	RevokeOtherSessions(ctx *contracts.RequestContext) bizerr.BizError
}

// userService 用户服务实现
type userService struct {
	DL        datalayer.UserDatalayer        `inject:""`
	RoleDL    datalayer.RoleDatalayer        `inject:""`
	FeatureDL datalayer.FeatureDatalayer     `inject:""`
	SessionDL datalayer.UserSessionDatalayer `inject:""`
	JWT       auroraFeature.JWTService       `inject:""`
	Redis     auroraFeature.RedisService     `inject:""`
	Geo       geoip.Resolver                 `inject:""`
	// TokenGuard 会话有效性内核(撤销 + 登录 IP 绑定):替代原 homeserver 自有的 tokenguard/tokenip/tokendeny 副本。
	TokenGuard tokenguard.Guard `inject:""`
	// LoginGuard 登录暴力破解防护(防护三件套之「登录前」)。本服务用**硬锁**模式(AcctLockSeconds>0):
	// 账号失败超阈值即锁账号、预检拦截(运维去 Redis 清)。account 传**明文 email**便于手动解锁。
	LoginGuard loginguard.Guard `inject:""`
}

// NewUserService 创建用户服务
func NewUserService(app contracts.App) UserService {
	c := &userService{}
	if err := app.Resolve(c); err != nil {
		panic(fmt.Errorf("failed to resolve UserService: %w", err))
	}
	return c
}
