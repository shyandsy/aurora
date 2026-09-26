package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/logger"
)

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
// 非法(未知等级 / 动作名不在该 scope 目录 / scope 未注册)→ 400 带原因。
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
