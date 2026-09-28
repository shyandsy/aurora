package auth

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/modules/user/model/dto"
	serviceUser "github.com/shyandsy/aurora/modules/user/service/user"
	"github.com/gin-gonic/gin"
)

// Logout 管理员登出:把当前 accessToken（Authorization 头）与 refreshToken（body）都拉黑。
// 该路由受 JWT 中间件保护。
func Logout(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req dto.LogoutReq
	// body 可选（可只带 accessToken 登出）；解析失败不阻断,按空 refreshToken 处理
	_ = c.ShouldBindJSON(&req)

	// Get UserService from DI container
	var userService serviceUser.UserService
	if err := c.App.Find(&userService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	if bizErr := userService.Logout(c, req); bizErr != nil {
		return nil, bizErr
	}

	return gin.H{"message": c.T("auth.logout_success")}, nil
}
