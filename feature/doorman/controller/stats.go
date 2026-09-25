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
