package controller

import (
	"strconv"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/logger"
)

// GET /stats?scope=&days= —— 决策统计(各风险等级命中数 / 各动作数 + 判定→完成漏斗;days<=0 默认 7)。
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

// GET /decisions?scope=&page=&pageSize= —— 决策明细(id 倒序,新→旧;页码翻页)。
// 返回 {items, total, page, pageSize};scope 空=全量,非空=只看该 scope;page 从 1 起,pageSize 服务层归一。
func handleDecisions(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	svc, e := resolveConsole(c)
	if e != nil {
		return nil, e
	}
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))
	res, err := svc.ListDecisionsPage(c.Query("scope"), page, pageSize)
	if err != nil {
		logger.Errorf("doorman: list decisions failed: %+v", err)
		return nil, bizerr.ErrInternalServerError(err)
	}
	return res, nil
}
