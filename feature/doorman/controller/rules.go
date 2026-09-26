package controller

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/feature/doorman"
	"github.com/shyandsy/aurora/logger"
)

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

// POST /rules(新增)/ PUT /rules/:id(更新)—— 保存前经 Console 校验(条件/scope 合法),非法 → 400 带原因。
func handleUpsertRule(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, e := resolveConsole(c)
	if e != nil {
		return nil, e
	}
	var dto doorman.RuleDTO
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
		// 校验类(参数非法/类别未知/scope 未注册)与不存在,都是客户端可纠正的错 → 400 带原因
		return nil, bizerr.NewValidationError(err.Error(), nil)
	}
	return out, nil
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
		if errors.Is(err, doorman.ErrRuleNotFound) {
			return nil, bizerr.NewValidationError(err.Error(), nil)
		}
		logger.Errorf("doorman: delete rule failed: %+v", err)
		return nil, bizerr.ErrInternalServerError(err)
	}
	return gin.H{"ok": true}, nil
}
