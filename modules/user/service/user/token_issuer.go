package user

import (
	"strings"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"

	"github.com/shyandsy/aurora/modules/user/model/dto"
	"github.com/shyandsy/aurora/modules/user/model/entity"
)

// token_issuer.go 是**唯一**的正式 token 签发中枢:普通登录、2FA 第二步、confirm 后自动登录全部委托到这里。
//
// 发号形态 = **feature-in-token**:登录时把角色权限「展开」成扁平的 feature 列表写进 JWT,
// 中间件按 feature 精确匹配鉴权、aurora 校验一行不改。展开只发生在此处(签发侧),token 因此自描述、
// 各服务零查库即可判权。role-in-token(把 roleID 写进 token、鉴权时经 Redis 读模型按角色展开)是可选的
// 另一形态,不在本服务当前范围内。
//
// 展开三步闭包见 datalayer.FeatureDatalayer.GetExpandedNamesByRoleID:
//  1. 角色 role_features 直授的 feature;
//  2. 每个 ui.page.* 经 ui_feature_grants 授予的业务/菜单 feature;
//  3. 对结果集每个 feature 并入 feature_dependencies 的传递闭包;超管角色直授 "*"。

// issueTokens 登录成功后的统一签发入口:展开 feature → 签 JWT → 绑 IP → 记会话。
func (s *userService) issueTokens(ctx *contracts.RequestContext, user *entity.User) (*dto.LoginResp, bizerr.BizError) {
	// 签发时展开:ui.page.*.view/operate → 业务 api feature + 跨页依赖 + ui.menu.*。
	// 展开只在此处发生,token 仍是扁平 feature 列表,中间件精确匹配不变。
	featureNames, err := s.FeatureDL.GetExpandedNamesByRoleID(ctx.Context, user.RoleID)
	if err != nil {
		return nil, internalErr(ctx, "user", err)
	}

	tokenResp, terr := s.JWT.GenerateToken(user.ID, user.Email, featureNames)
	if terr != nil {
		return nil, internalErr(ctx, "user", terr)
	}

	// web token 绑登录 IP(refresh 侧 fail-close 兜底);记一条会话(best-effort,失败不阻断登录)。
	s.bindTokenIP(ctx, tokenResp)
	s.recordSession(ctx, user.ID, tokenResp, clientTypeOf(ctx))

	userDto := user.ToDto()
	userDto.Features = featureNames

	return &dto.LoginResp{
		AccessToken:     tokenResp.AccessToken,
		TokenType:       "bearer",
		ExpiresInSecond: tokenResp.ExpiresIn,
		RefreshToken:    tokenResp.RefreshToken,
		Features:        featureNames,
		User:            userDto,
	}, nil
}

// clientTypeOf 取客户端类型(会话记录用),缺省 web。由请求头 X-Client-Type 声明(web/app/...)。
func clientTypeOf(ctx *contracts.RequestContext) string {
	ct := strings.TrimSpace(ctx.GetHeader("X-Client-Type"))
	if ct == "" {
		return clientTypeWeb
	}
	return ct
}
