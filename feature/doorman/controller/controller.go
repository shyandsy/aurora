// Package controller 是 doorman 的管理 API(配置页后端)HTTP 层:把 doorman.Console 暴露成路由。
// 独立子包,依赖 doorman(用 doorman.Console / 各 DTO);handler 按资源分文件:schema / rules / policy / stats。
package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/feature/doorman"
	"github.com/shyandsy/aurora/logger"
)

// Routes 返回 doorman 管理 API 的全部路由,所有端点共用同一套鉴权中间件 mws。
// prefix 例:"/api/admin/v1/doorman"。适合「看决策」与「管规则」同权限的简单接法。
// 想把两者分权(如只让运营看决策、不让改规则),用 RoutesWithObserve。
//
//	import doormanctl "github.com/shyandsy/aurora/feature/doorman/controller"
//	app.RegisterRoutes(append(myRoutes, doormanctl.Routes(prefix, myAdminAuth)...))
func Routes(prefix string, mws ...gin.HandlerFunc) []contracts.Route {
	return RoutesWithObserve(prefix, mws, mws)
}

// RoutesWithObserve 把 doorman API 按两档权限分别挂载:
//   - manage:配置类端点(kinds / scopes / rules / policy)—— 改规则/策略,权限最高。
//   - observe:观测类端点(stats / decisions)—— 只看决策统计与台账,不碰配置。
//
// 为什么分:配置页(./Config 组件)需要 manage;而决策明细弹窗(./Decisions 组件)可以被
// **任意宿主页面**打开(见 web/README),给「只看决策」的角色(如运营)单独放行 observe 即可,
// 无需授予能改规则的 manage。doorman **保持中立**:不认识任何业务权限名,manage/observe 具体
// 由宿主的 RBAC 中间件决定、映射到宿主自己的 feature。observe 传 nil → 退化为与 manage 同权。
func RoutesWithObserve(prefix string, manage, observe []gin.HandlerFunc) []contracts.Route {
	if observe == nil {
		observe = manage
	}
	return []contracts.Route{
		// —— 配置类(manage):schema + 规则 CRUD + 风险→动作策略 ——
		{Method: "GET", Path: prefix + "/kinds", Handler: handleKinds, Middlewares: manage},
		{Method: "GET", Path: prefix + "/scopes", Handler: handleScopes, Middlewares: manage},
		{Method: "GET", Path: prefix + "/rules", Handler: handleListRules, Middlewares: manage},
		{Method: "POST", Path: prefix + "/rules", Handler: handleUpsertRule, Middlewares: manage},
		{Method: "PUT", Path: prefix + "/rules/:id", Handler: handleUpsertRule, Middlewares: manage},
		{Method: "DELETE", Path: prefix + "/rules/:id", Handler: handleDeleteRule, Middlewares: manage},
		{Method: "GET", Path: prefix + "/policy/kinds", Handler: handlePolicyKinds, Middlewares: manage},
		{Method: "GET", Path: prefix + "/policy", Handler: handleGetPolicy, Middlewares: manage},
		{Method: "PUT", Path: prefix + "/policy", Handler: handleSetPolicy, Middlewares: manage},
		// —— 观测类(observe):只读决策统计 + 明细(页码翻页),可单独授「看决策」权限 ——
		{Method: "GET", Path: prefix + "/stats", Handler: handleStats, Middlewares: observe},
		{Method: "GET", Path: prefix + "/decisions", Handler: handleDecisions, Middlewares: observe},
	}
}

// resolveConsole 每个 handler 开头拿管理服务:从请求的 DI 容器 Find 出 doorman.Console。
// 这是 aurora 惯例——handler 是无状态函数,按请求从 ctx.App 解析依赖,不做注入式 controller 结构体
// (见 aurora sample/full_showcase/controller 与各服务 controller)。低频 admin API,Find 开销可忽略。
func resolveConsole(c *contracts.RequestContext) (doorman.Console, bizerr.BizError) {
	var svc doorman.Console
	if err := c.App.Find(&svc); err != nil {
		logger.Errorf("doorman: resolve Console failed: %+v", err)
		return nil, bizerr.ErrInternalServerError(err)
	}
	return svc, nil
}
