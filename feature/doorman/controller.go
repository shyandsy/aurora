package doorman

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/logger"
)

// Routes 返回 doorman 规则管理 API 的路由。业务把它挂到自己的 admin 路由组下、**自带鉴权中间件**(mws)。
// prefix 例:"/api/admin/v1/doorman"。这套是配置页(web/doorman 微前端)的后端。
//
//	app.RegisterRoutes(append(myRoutes, doorman.Routes(prefix, myAdminAuth)...))
func Routes(prefix string, mws ...gin.HandlerFunc) []contracts.Route {
	return []contracts.Route{
		{Method: "GET", Path: prefix + "/kinds", Handler: handleKinds, Middlewares: mws},
		{Method: "GET", Path: prefix + "/scopes", Handler: handleScopes, Middlewares: mws},
		{Method: "GET", Path: prefix + "/rules", Handler: handleListRules, Middlewares: mws},
		{Method: "POST", Path: prefix + "/rules", Handler: handleUpsertRule, Middlewares: mws},
		{Method: "PUT", Path: prefix + "/rules/:id", Handler: handleUpsertRule, Middlewares: mws},
		{Method: "DELETE", Path: prefix + "/rules/:id", Handler: handleDeleteRule, Middlewares: mws},
		// 「风险等级 → 动作」策略(配置页下半:每个等级配走哪个动作)。
		{Method: "GET", Path: prefix + "/policy/kinds", Handler: handlePolicyKinds, Middlewares: mws},
		{Method: "GET", Path: prefix + "/policy", Handler: handleGetPolicy, Middlewares: mws},
		{Method: "PUT", Path: prefix + "/policy", Handler: handleSetPolicy, Middlewares: mws},
		// 决策统计(配置页看板:各风险等级命中数 / 各动作数)。
		{Method: "GET", Path: prefix + "/stats", Handler: handleStats, Middlewares: mws},
		// 决策明细(配置页明细表:时间/命中规则/风险/动作/UA·IP·ASN;游标翻页)。
		{Method: "GET", Path: prefix + "/decisions", Handler: handleDecisions, Middlewares: mws},
	}
}

func resolveConsole(c *contracts.RequestContext) (Console, bizerr.BizError) {
	var svc Console
	if err := c.App.Find(&svc); err != nil {
		logger.Errorf("doorman: resolve Console failed: %+v", err)
		return nil, bizerr.ErrInternalServerError(err)
	}
	return svc, nil
}

// GET /kinds —— 条件/动作类别 + 各自配置字段 schema(配置页据此动态渲染)。
func handleKinds(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, e := resolveConsole(c)
	if e != nil {
		return nil, e
	}
	return svc.Kinds(), nil
}

// GET /scopes —— 已注册的 scope 定义(id+标签+动作目录);配置页据此数据驱动 tab 与「风险→动作」下拉。
func handleScopes(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, e := resolveConsole(c)
	if e != nil {
		return nil, e
	}
	return svc.Scopes(), nil
}

// GET /rules?scope= —— 列某 scope 的规则(scope 空 = 全部)。
func handleListRules(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, e := resolveConsole(c)
	if e != nil {
		return nil, e
	}
	rules, err := svc.ListRules(c.Query("scope"))
	if err != nil {
		logger.Errorf("doorman: list rules failed: %+v", err)
		return nil, bizerr.ErrInternalServerError(err)
	}
	return rules, nil
}

// POST /rules(新增)/ PUT /rules/:id(更新)—— 保存前经 Console 校验,非法 → 400 带原因。
func handleUpsertRule(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, e := resolveConsole(c)
	if e != nil {
		return nil, e
	}
	var dto RuleDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		return nil, bizerr.NewValidationError(c.T("error.bad_request"), nil)
	}
	if idStr := c.Param("id"); idStr != "" { // PUT 带路径 id,覆盖 body
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, bizerr.NewValidationError(c.T("error.bad_request"), nil)
		}
		dto.ID = id
	}
	out, err := svc.UpsertRule(dto)
	if err != nil {
		// 校验类(参数非法/类别未知)与不存在,都是客户端可纠正的错 → 400 带原因
		return nil, bizerr.NewValidationError(err.Error(), nil)
	}
	return out, nil
}

// GET /policy/kinds?scope= —— 风险等级枚举 + 该 scope 可选动作名(配置页「风险→动作」下拉)。
func handlePolicyKinds(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, e := resolveConsole(c)
	if e != nil {
		return nil, e
	}
	return svc.PolicyKinds(c.Query("scope")), nil
}

// GET /policy?scope= —— 当前「风险等级 → 动作名」映射。
func handleGetPolicy(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, e := resolveConsole(c)
	if e != nil {
		return nil, e
	}
	m, err := svc.GetPolicy(c.Query("scope"))
	if err != nil {
		logger.Errorf("doorman: get policy failed: %+v", err)
		return nil, bizerr.ErrInternalServerError(err)
	}
	if m == nil {
		m = map[string]string{}
	}
	return m, nil
}

// PUT /policy?scope= —— 整体覆盖某 scope 的策略。body: {"critical":"block","high":"email_verify",...}。
// 非法(未知等级 / 动作名不在该 scope 目录)→ 400 带原因。
func handleSetPolicy(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, e := resolveConsole(c)
	if e != nil {
		return nil, e
	}
	scope := c.Query("scope")
	if scope == "" {
		return nil, bizerr.NewValidationError(c.T("error.bad_request"), nil)
	}
	var mapping map[string]string
	if err := c.ShouldBindJSON(&mapping); err != nil {
		return nil, bizerr.NewValidationError(c.T("error.bad_request"), nil)
	}
	if err := svc.SetPolicy(scope, mapping); err != nil {
		return nil, bizerr.NewValidationError(err.Error(), nil)
	}
	return gin.H{"ok": true}, nil
}

// GET /stats?scope=&days= —— 决策统计(各风险等级命中数 / 各动作数;days<=0 默认 7)。
func handleStats(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, e := resolveConsole(c)
	if e != nil {
		return nil, e
	}
	days, _ := strconv.Atoi(c.Query("days"))
	st, err := svc.Stats(c.Query("scope"), days)
	if err != nil {
		logger.Errorf("doorman: stats failed: %+v", err)
		return nil, bizerr.ErrInternalServerError(err)
	}
	return st, nil
}

// GET /decisions?scope=&limit=&before= —— 决策明细(id 倒序;before=上一页最后一条 id,游标翻页)。
func handleDecisions(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, e := resolveConsole(c)
	if e != nil {
		return nil, e
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	before, _ := strconv.ParseInt(c.Query("before"), 10, 64)
	rows, err := svc.ListDecisions(c.Query("scope"), limit, before)
	if err != nil {
		logger.Errorf("doorman: list decisions failed: %+v", err)
		return nil, bizerr.ErrInternalServerError(err)
	}
	return rows, nil
}

// DELETE /rules/:id
func handleDeleteRule(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, e := resolveConsole(c)
	if e != nil {
		return nil, e
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return nil, bizerr.NewValidationError(c.T("error.bad_request"), nil)
	}
	if err := svc.DeleteRule(id); err != nil {
		if errors.Is(err, ErrRuleNotFound) {
			return nil, bizerr.NewValidationError(err.Error(), nil)
		}
		logger.Errorf("doorman: delete rule failed: %+v", err)
		return nil, bizerr.ErrInternalServerError(err)
	}
	return gin.H{"ok": true}, nil
}
