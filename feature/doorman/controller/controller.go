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

// Routes 返回 doorman 管理 API 的路由。业务把它挂到自己的 admin 路由组下、**自带鉴权中间件**(mws)。
// prefix 例:"/api/admin/v1/doorman"。这套是配置页(schema 驱动的前端组件)的后端。
//
//	import doormanctl "github.com/shyandsy/aurora/feature/doorman/controller"
//	app.RegisterRoutes(append(myRoutes, doormanctl.Routes(prefix, myAdminAuth)...))
func Routes(prefix string, mws ...gin.HandlerFunc) []contracts.Route {
	return []contracts.Route{
		// schema —— 配置页据此动态渲染(条件类别/字段 + 已注册 scope/动作目录)。
		{Method: "GET", Path: prefix + "/kinds", Handler: handleKinds, Middlewares: mws},
		{Method: "GET", Path: prefix + "/scopes", Handler: handleScopes, Middlewares: mws},
		// 规则 CRUD。
		{Method: "GET", Path: prefix + "/rules", Handler: handleListRules, Middlewares: mws},
		{Method: "POST", Path: prefix + "/rules", Handler: handleUpsertRule, Middlewares: mws},
		{Method: "PUT", Path: prefix + "/rules/:id", Handler: handleUpsertRule, Middlewares: mws},
		{Method: "DELETE", Path: prefix + "/rules/:id", Handler: handleDeleteRule, Middlewares: mws},
		// 「风险等级 → 动作」策略(每个等级配走哪个动作)。
		{Method: "GET", Path: prefix + "/policy/kinds", Handler: handlePolicyKinds, Middlewares: mws},
		{Method: "GET", Path: prefix + "/policy", Handler: handleGetPolicy, Middlewares: mws},
		{Method: "PUT", Path: prefix + "/policy", Handler: handleSetPolicy, Middlewares: mws},
		// 决策统计(看板)+ 明细(游标翻页)。
		{Method: "GET", Path: prefix + "/stats", Handler: handleStats, Middlewares: mws},
		{Method: "GET", Path: prefix + "/decisions", Handler: handleDecisions, Middlewares: mws},
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
