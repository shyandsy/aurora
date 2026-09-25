// Package scope 定义「客户端作用域(scope)」在 token 上的自描述约定(tokenguard 内部实现)。
//
// scope 的策略数据(有哪些 scope、绑不绑 IP)由签发方从自己的数据源读取;一旦签发,token 通过
// features 里的命名空间标记自描述,使共享鉴权中间件无需访问任何数据库即可施策:
//   - `scope:<name>` —— 本 token 属于哪个作用域(缺省视为 web,兼容无标记的历史 token);
//   - `sess:noip`    —— 本 token 跳过 IP 绑定校验(由签发方按 scope.bind_ip 决定是否打上)。
//
// 对外经 tokenguard 的 SessionTags / ScopeOf / PublicFeatures 暴露,本包不直接对外。
package scope

import (
	"slices"
	"strings"
)

const (
	tagScopePrefix = "scope:"    // features 里的 scope 标记:scope:<name>
	tagSessionNoIP = "sess:noip" // features 里的会话策略标记:跳过 IP 绑定

	// DefaultScope 无 scope 标记的历史 token 归入的作用域(web:绑 IP、可达管理面)。
	DefaultScope = "web"
)

// Tag 返回某作用域名对应的 features 标记(签发时写入)。
func Tag(name string) string { return tagScopePrefix + name }

// NoIPTag 返回"跳过 IP 绑定"的 features 标记。
func NoIPTag() string { return tagSessionNoIP }

// Of 从 features 解析 token 的作用域名;无标记返回 DefaultScope。
func Of(features []string) string {
	for _, f := range features {
		if name, ok := strings.CutPrefix(f, tagScopePrefix); ok {
			return name
		}
	}
	return DefaultScope
}

// SkipIPBinding 该 token 是否跳过 IP 绑定。
func SkipIPBinding(features []string) bool {
	return slices.Contains(features, tagSessionNoIP)
}

// IsInternalTag 供签发方过滤:对外返回给客户端的 features 不含这些内部标记。
func IsInternalTag(f string) bool {
	return strings.HasPrefix(f, tagScopePrefix) || f == tagSessionNoIP
}
