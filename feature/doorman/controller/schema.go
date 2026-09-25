package controller

import (
	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
)

// GET /kinds —— 条件/动作类别 + 各自配置字段 schema(配置页据此动态渲染表单)。
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
