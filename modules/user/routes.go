package user

import (
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/middleware"
	"github.com/shyandsy/aurora/modules/user/controller/auth"
	"github.com/shyandsy/aurora/modules/user/controller/feature"
	"github.com/shyandsy/aurora/modules/user/controller/microservice"
	ratelimitctrl "github.com/shyandsy/aurora/modules/user/controller/ratelimit"
	"github.com/shyandsy/aurora/modules/user/controller/role"
	"github.com/shyandsy/aurora/modules/user/controller/role_feature"
	"github.com/shyandsy/aurora/modules/user/controller/user"
	"github.com/gin-gonic/gin"
)

// Routes 返回 user 服务（用户中心）的所有路由。
// user 自立、自签 JWT + 自带 RBAC；受保护路由按 feature 粒度用 JWTAuthMiddleware 校验。
// JWT 通过共享 secret 与后台域其它服务互通（同域服务复用同一把 secret 校验，不查 user 库）。
//
// (逐字搬自原 controller/routes.go:GetRoutes;路径 / handler / 中间件 / 权限点未做任何改动。)
func Routes(app contracts.App) []contracts.Route {
	serviceName := app.Name()
	apiPrefix := "/api/" + serviceName + "/v1"

	jwt := func() []gin.HandlerFunc {
		return []gin.HandlerFunc{middleware.JWTAuthMiddleware(app)}
	}
	// jwtEnroll:放行「仅绑定 2FA」受限 token —— 只给 2FA 绑定闭环 + 登出用,让未绑定用户
	// 能完成绑定(绑定成功换正式 token)或退出,除此之外的接口一律被普通 jwt() 硬拦。
	jwtEnroll := func() []gin.HandlerFunc {
		return []gin.HandlerFunc{middleware.JWTAuthMiddlewareAllowEnroll(app)}
	}

	return []contracts.Route{
		// ==== 用户中心：登录 / 用户 / 角色 / 权限（自立，自签 JWT）====
		// ---- Auth（登录/登出/刷新）----
		// 登录：公开。登录暴力破解防护(按 IP + 账号失败计数/锁/每小时上限)已由 aurora loginguard
		// 在 service 层的权威判定点做(precheckLogin + Record*),不再挂旧的 LoginRateLimitMiddleware
		// ——两者打的是同一件事,并存会双限流。见 service/user/loginguard.go。
		{
			Method:  "POST",
			Path:    apiPrefix + "/auth/login",
			Handler: auth.Login,
		},
		// 登出：需登录，把当前 access + refresh token 拉黑（Redis 黑名单，立即失效）。
		// 放行仅绑定 token —— 未绑定用户也能退出。
		{Method: "POST", Path: apiPrefix + "/auth/logout", Handler: auth.Logout, Middlewares: jwtEnroll()},
		// 刷新：无需 JWT，用 refreshToken 换新的 access+refresh，支撑前端 401 自动续期
		{Method: "POST", Path: apiPrefix + "/auth/refresh", Handler: auth.Refresh},
		// 后台「下载门禁」的 forwardAuth 目标（无需 JWT · 它自己就是那道校验）。
		// 读 admin_gate cookie 校验：有效→200 放行下发 SPA；否则→302 跳登录壳 /gate/。
		// 只管「是否下发大包」，不授权任何数据访问（各 API 端点仍各自校验 JWT+权限）。见 auth/gate.go。
		// 相位 A：本端点已备好，但 forwardAuth 仍指向 admin verify（不锁人）；相位 B 才翻到这里（纯改 values）。
		{Method: "GET", Path: apiPrefix + "/auth/gate/verify", Handler: auth.GateVerify},
		// 两步验证登录第二步：公开;限流同登录,由 loginguard 在 service 层做(LoginTwoFactor 内 precheckLogin + Record*)。
		{Method: "POST", Path: apiPrefix + "/auth/login/2fa", Handler: auth.LoginTwoFactor},
		// 两步验证绑定/确认：放行仅绑定 token —— 未绑定用户正是靠这两个端点完成绑定(confirm 成功换正式 token)。
		{Method: "POST", Path: apiPrefix + "/auth/2fa/setup", Handler: auth.SetupTotp, Middlewares: jwtEnroll()},
		{Method: "POST", Path: apiPrefix + "/auth/2fa/confirm", Handler: auth.ConfirmTotp, Middlewares: jwtEnroll()},
		// 关闭两步验证：需正式登录态(已绑定用户才有得关),仅绑定 token 不放行。
		{Method: "POST", Path: apiPrefix + "/auth/2fa/disable", Handler: auth.DisableTotp, Middlewares: jwt()},

		// ---- 登录会话/设备清单(本人,受保护)：列出 / 撤销 / 撤销其余 ----
		{Method: "GET", Path: apiPrefix + "/auth/sessions", Handler: auth.ListSessions, Middlewares: jwt()},
		{Method: "POST", Path: apiPrefix + "/auth/sessions/revoke-others", Handler: auth.RevokeOtherSessions, Middlewares: jwt()},
		{Method: "POST", Path: apiPrefix + "/auth/sessions/:sid/revoke", Handler: auth.RevokeSession, Middlewares: jwt()},

		// ---- User（受保护，按 feature 粒度鉴权）----
		{Method: "GET", Path: apiPrefix + "/user", Handler: user.GetUsers, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "user.get")}},
		{Method: "GET", Path: apiPrefix + "/user/:id", Handler: user.GetUser, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "user.get")}},
		{Method: "POST", Path: apiPrefix + "/user", Handler: user.CreateUser, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "user.create")}},
		{Method: "PUT", Path: apiPrefix + "/user/:id", Handler: user.UpdateUser, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "user.update")}},
		{Method: "DELETE", Path: apiPrefix + "/user/:id", Handler: user.DeleteUser, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "user.delete")}},

		// ---- 登录被锁管理（受保护）：列出被 loginguard 锁的 IP/账号、手动解锁。复用用户权限:查看=user.get,解锁=user.update ----
		{Method: "GET", Path: apiPrefix + "/rate-limit/locked", Handler: ratelimitctrl.ListLocked, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "user.get")}},
		{Method: "POST", Path: apiPrefix + "/rate-limit/locked/unlock", Handler: ratelimitctrl.Unlock, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "user.update")}},

		// ---- Role（受保护）----
		{Method: "GET", Path: apiPrefix + "/role", Handler: role.GetRoles, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "role.get")}},
		{Method: "GET", Path: apiPrefix + "/role/:id", Handler: role.GetRole, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "role.get")}},
		{Method: "POST", Path: apiPrefix + "/role", Handler: role.CreateRole, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "role.create")}},
		{Method: "PUT", Path: apiPrefix + "/role/:id", Handler: role.UpdateRole, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "role.update")}},
		{Method: "DELETE", Path: apiPrefix + "/role/:id", Handler: role.DeleteRole, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "role.delete")}},

		// ---- Feature（受保护，只读）----
		{Method: "GET", Path: apiPrefix + "/feature", Handler: feature.GetFeatures, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "feature.get")}},
		{Method: "GET", Path: apiPrefix + "/feature/:id", Handler: feature.GetFeature, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "feature.get")}},

		// ---- RoleFeature（受保护）----
		{Method: "GET", Path: apiPrefix + "/role-feature", Handler: role_feature.GetRoleFeatures, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "rolefeature.get")}},
		{Method: "GET", Path: apiPrefix + "/role-feature/:id", Handler: role_feature.GetRoleFeature, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "rolefeature.get")}},
		{Method: "POST", Path: apiPrefix + "/role-feature", Handler: role_feature.CreateRoleFeature, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "rolefeature.create")}},
		{Method: "DELETE", Path: apiPrefix + "/role-feature/:id", Handler: role_feature.DeleteRoleFeature, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "rolefeature.delete")}},

		// ---- Microservice token 功能配置(受保护,按 feature 粒度鉴权)----
		// 预定义「微服务身份 + 其所需 feature 列表」的增删改查。
		{Method: "GET", Path: apiPrefix + "/microservice", Handler: microservice.GetMicroserviceTokenFeatures, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "microservice.get")}},
		{Method: "GET", Path: apiPrefix + "/microservice/:id", Handler: microservice.GetMicroserviceTokenFeature, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "microservice.get")}},
		{Method: "POST", Path: apiPrefix + "/microservice", Handler: microservice.CreateMicroserviceTokenFeature, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "microservice.create")}},
		{Method: "PUT", Path: apiPrefix + "/microservice/:id", Handler: microservice.UpdateMicroserviceTokenFeature, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "microservice.update")}},
		{Method: "DELETE", Path: apiPrefix + "/microservice/:id", Handler: microservice.DeleteMicroserviceTokenFeature, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "microservice.delete")}},

		// ---- Microservice JWT token 签发/查询/启停(受保护,按 feature 粒度鉴权)----
		// 签发的是「服务间调用」长效 access token,走 aurora Claims + TokenType=access,同域 JWT_SECRET 本地签名。
		{Method: "POST", Path: apiPrefix + "/microservice/jwt_token", Handler: microservice.IssueJWTToken, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "microservice.jwt_token.issue")}},
		{Method: "GET", Path: apiPrefix + "/microservice/jwt_token", Handler: microservice.GetJWTTokens, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "microservice.jwt_token.get")}},
		{Method: "GET", Path: apiPrefix + "/microservice/jwt_token/:id", Handler: microservice.GetJWTToken, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "microservice.jwt_token.get")}},
		{Method: "POST", Path: apiPrefix + "/microservice/jwt_token/:id/enable", Handler: microservice.EnableToken, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "microservice.jwt_token.update")}},
		{Method: "POST", Path: apiPrefix + "/microservice/jwt_token/:id/disable", Handler: microservice.DisableToken, Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "microservice.jwt_token.update")}},
	}
}
