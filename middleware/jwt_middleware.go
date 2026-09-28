package middleware

import (
	"context"
	"strings"

	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
	"github.com/shyandsy/aurora/logger"
	"github.com/gin-gonic/gin"

	"github.com/shyandsy/aurora/middleware/rolefeature"
)

const (
	// ContextKeyUserID is the key for storing user ID in gin.Context
	ContextKeyUserID = "user_id"
	// ContextKeyUserEmail is the key for storing user email in gin.Context
	ContextKeyUserEmail = "user_email"
	// ContextKeyRequiredFeature is the key for storing required feature in gin.Context
	ContextKeyRequiredFeature = "required_feature"
	// ContextKeyIssuedAt 存 token 的签发时刻(iat,unix 秒),供 RequireLiveSession 判「凭据变更纪元」
	// (改密/重置后 iat<epoch 的旧 token 一律作废)。
	ContextKeyIssuedAt = "token_iat"

	// ScopeEnroll2FA 是「仅绑定两步验证」token 的哨兵特征名。
	//
	// 强制两步验证开启、但账号尚未绑定时,登录发的是**带此哨兵**的受限 token(见 admin login.go):
	// 它能走 2FA 绑定/登出端点,但被普通鉴权中间件**硬拦在所有其它接口之外**。把「必须先绑定」
	// 从前端提示升级为**后端强制** —— 否则空权限 token 会畅通无阻地过任何「不做 feature 粒度校验」
	// 的无 feature 路由(哪天新增一条无 feature 路由忘挂 feature,TOTP 强制就被静默绕过)。
	// 双下划线包裹,绝不与真实 feature 名冲突。
	ScopeEnroll2FA = "__enroll_2fa_only__"
)

// isEnrollOnlyToken 判断 claims.Features 是否为「仅绑定 2FA」受限 token(含 ScopeEnroll2FA 哨兵)。
func isEnrollOnlyToken(features []string) bool {
	for _, f := range features {
		if f == ScopeEnroll2FA {
			return true
		}
	}
	return false
}

// hasRequiredFeature 判断有效 feature 集是否满足所需 feature。
// "*" 是超管通配:持有即视为拥有一切权限(admin 角色 seed 一条,见迁移 20260816130000)。
// 新增 feature 无需再逐个 grant 给 admin。此函数与改造前的内联判定逐位等价。
func hasRequiredFeature(features []string, requiredFeature string) bool {
	for _, f := range features {
		if f == requiredFeature || f == "*" {
			return true
		}
	}
	return false
}

// effectiveFeatures 计算权限判定用的「有效 feature 集」= claims.Features ∪(Roles 展开)。
//
// 红线:本批所有 token 的 claims.Roles 恒为空 —— 此时**直接返回 claims.Features 原样**,
// 既不查读模型也不碰 Redis/DI,判定路径与旧逻辑逐位一致(admin 生产鉴权行为不变)。
//
// 仅当 claims.Roles 非空(未来 role-in-token 批次)才对每个 role 查共享读模型
// (common/middleware/rolefeature)展开成 feature,**并入** claims.Features(取并集)。
// fail-close:Redis 后端整体不可用 → 所有 role 贡献 0(只保留 token 直载的 Features);
// 单个 role 查不到 / 出错 → 该 role 贡献 0(见 resolveEffectiveFeatures)。均记日志、绝不 fall-through 放行。
func effectiveFeatures(app contracts.App, ctx context.Context, claims *auroraFeature.Claims) []string {
	if len(claims.Roles) == 0 {
		return claims.Features // 红线:Roles 为空 → 原路径,一位不变
	}
	var store auroraFeature.RedisService
	if err := app.Find(&store); err != nil {
		// 读模型后端拿不到 → 所有 role 贡献 0(fail-close)。仍返回 token 直载 feature,由判定收尾。
		logger.Errorf("[rolefeature] 读模型 Redis 不可用,%d 个 role 全部 fail-close 贡献 0 feature: %v", len(claims.Roles), err)
		return claims.Features
	}
	return resolveEffectiveFeatures(ctx, store, claims)
}

// resolveEffectiveFeatures 是 effectiveFeatures 的纯函数内核(store 已就绪,便于单测)。
// 起点是 token 直载的 claims.Features(密码学可信、不依赖读模型),再逐 role 并入读模型展开;
// 某 role 查不到 / 后端错 → 记日志、该 role 贡献 0,继续下一个 role,绝不放行。
func resolveEffectiveFeatures(ctx context.Context, store rolefeature.Store, claims *auroraFeature.Claims) []string {
	if len(claims.Roles) == 0 {
		return claims.Features // 红线:Roles 为空 → 原样返回
	}
	out := make([]string, 0, len(claims.Features)+len(claims.Roles))
	out = append(out, claims.Features...)
	for _, roleName := range claims.Roles {
		set, err := rolefeature.ResolveFeatures(ctx, store, []string{roleName})
		if err != nil {
			logger.Errorf("[rolefeature] role %q 展开失败(fail-close,贡献 0 feature): %v", roleName, err)
			continue
		}
		for f := range set {
			out = append(out, f)
		}
	}
	return out
}

// JWTAuthMiddleware:鉴权 +(可选)feature 校验。默认**拒绝**「仅绑定 2FA」受限 token ——
// 除 2FA 绑定闭环端点外,所有接口都用它(是默认)。
func JWTAuthMiddleware(app contracts.App, feature ...string) gin.HandlerFunc {
	return jwtAuth(app, false, feature...)
}

// JWTAuthMiddlewareAllowEnroll:鉴权,但**放行**「仅绑定 2FA」受限 token。
// 只给 2FA 绑定闭环用的少数端点挂(/auth/2fa/setup、/auth/2fa/confirm、/auth/logout)——
// 让未绑定用户能完成绑定(confirm 成功后换发正式 token)或登出,除此之外一律被 JWTAuthMiddleware 拦死。
func JWTAuthMiddlewareAllowEnroll(app contracts.App, feature ...string) gin.HandlerFunc {
	return jwtAuth(app, true, feature...)
}

// jwtAuth 实现:从 Authorization 头取 token、校验、把用户信息塞进 context;配了 feature 则校验权限。
// allowEnroll 决定是否放行「仅绑定 2FA」受限 token(仅 2FA 绑定闭环端点为 true)。
func jwtAuth(app contracts.App, allowEnroll bool, feature ...string) gin.HandlerFunc {
	requiredFeature := ""
	if len(feature) > 0 && feature[0] != "" {
		requiredFeature = feature[0]
	}

	return func(c *gin.Context) {
		// Get JWT service from DI container
		var jwtService auroraFeature.JWTService
		if err := app.Find(&jwtService); err != nil {
			c.JSON(500, gin.H{
				"message": "JWT service not available",
			})
			c.Abort()
			return
		}

		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{
				"message": "Authorization header is required",
			})
			c.Abort()
			return
		}

		// Check if it's a Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(401, gin.H{
				"message": "Invalid authorization header format. Expected: Bearer <token>",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Validate token
		claims, err := jwtService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(401, gin.H{
				"message": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// 「仅绑定 2FA」token 硬拦:强制两步验证下、未绑定账号拿到的受限 token,除放行端点外一律 403。
		// 必须在 feature 校验**之前**、且不受 requiredFeature 是否为空影响 —— 否则「无 feature 路由」
		// 会跳过下面的权限校验、把这张受限 token 放进去(这正是它能过 dashboard 之类无 feature 接口的根)。
		// 一处堵死,胜过逐路由补 feature;并把「必须先绑定」从前端提示升级为后端强制。
		if isEnrollOnlyToken(claims.Features) && !allowEnroll {
			c.JSON(403, gin.H{
				"message":             "请先完成两步验证绑定后再操作",
				"mustEnrollTwoFactor": true,
			})
			c.Abort()
			return
		}

		// Check feature permission if required
		if requiredFeature != "" {
			// 双模有效 feature 集:本批 token 的 Roles 恒为空 → 有效集就是 claims.Features,
			// 判定与旧逻辑逐位一致(红线);仅 Roles 非空(未来 role-in-token)才叠加读模型展开(fail-close)。
			if !hasRequiredFeature(effectiveFeatures(app, c.Request.Context(), claims), requiredFeature) {
				// Return 403 Forbidden when user is authenticated but lacks required feature
				c.JSON(403, gin.H{
					"message": "Insufficient permissions",
				})
				c.Abort()
				return
			}
		}

		// Store user information in context for use in handlers
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUserEmail, claims.Email)
		if claims.IssuedAt != nil {
			c.Set(ContextKeyIssuedAt, claims.IssuedAt.Time.Unix()) // 供 RequireLiveSession 判凭据变更纪元
		}
		if requiredFeature != "" {
			c.Set(ContextKeyRequiredFeature, requiredFeature)
		}

		// Continue to next handler
		c.Next()
	}
}

// GetUserID extracts user ID from gin.Context (set by JWTAuthMiddleware)
func GetUserID(c *gin.Context) (int64, bool) {
	userID, exists := c.Get(ContextKeyUserID)
	if !exists {
		return 0, false
	}

	id, ok := userID.(int64)
	return id, ok
}

// GetUserEmail extracts user email from gin.Context (set by JWTAuthMiddleware)
func GetUserEmail(c *gin.Context) (string, bool) {
	email, exists := c.Get(ContextKeyUserEmail)
	if !exists {
		return "", false
	}

	emailStr, ok := email.(string)
	return emailStr, ok
}
