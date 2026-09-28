package microservice

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/types"
	serviceMicroserviceToken "github.com/shyandsy/aurora/modules/user/service/microservice_token"
)

// GetJWTTokens 分页获取微服务 token 记录
func GetJWTTokens(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	var req types.PagingReq
	if err := c.ShouldBindQuery(&req); err != nil {
		msg := c.T("error.bad_request")
		return nil, bizerr.NewValidationError(msg, nil)
	}

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	var tokenService serviceMicroserviceToken.MicroserviceTokenService
	if err := c.App.Find(&tokenService); err != nil {
		return nil, bizerr.ErrInternalServerError(err)
	}

	resp, bizErr := tokenService.GetMicroserviceTokenFeatureTokens(c, req)
	if bizErr != nil {
		return nil, bizErr
	}

	return resp, nil
}
