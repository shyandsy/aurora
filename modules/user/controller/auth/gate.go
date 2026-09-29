package auth

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/shyandsy/aurora/bizerr"
	"github.com/shyandsy/aurora/contracts"
	auroraFeature "github.com/shyandsy/aurora/feature"
)

// gateCookieName 是「后台下载门禁」cookie 的名字。它只是 SPA 当前 access token 的一份镜像
// (由前端 storage.service 在写/清 token 时同步维护),因为 Traefik 的 forwardAuth 只能看到
// cookie、看不到 SPA 放在 localStorage 里、通过 Authorization 头发的 token。
//
// ⚠️ cookie 名默认 "admin_gate":SPA 侧 storage.service 与登录壳都写这个名字,谁签发 token
// (admin 还是 user)与 cookie 名无关 —— 相位 A 把登录切到 user,但 cookie 镜像逻辑一行不动。
// 每个接入项目的 cookie 名可能不同,故设为可由模块装配层(user 模块)通过 SetGateCookieName 覆盖的变量。
var gateCookieName = "admin_gate"

// SetGateCookieName 设置门禁 cookie 名(供模块装配层按 Config 注入)。
// 传空串则保持当前值不变(默认 "admin_gate")——避免误清空。
func SetGateCookieName(name string) {
	if name != "" {
		gateCookieName = name
	}
}

// gateShellPath 是未通过门禁时要跳去的「登录壳」路径(一个极小的独立登录页,不含大 SPA)。
const gateShellPath = "/gate/"

// GateVerify 是 admin SPA「下载门禁」的 Traefik forwardAuth 目标。
//
// ⚠️ 它只授权**是否下发 Angular 大包**,不授权任何数据/接口访问 —— 每个 API 端点仍各自
// 强制校验 JWT + 权限点(见 JWTAuthMiddleware),门禁被绕过也拿不到任何数据。它的唯一目的是:
// 让未登录的陌生人只能拿到那个极小的登录壳,而**下载不到完整 SPA**(避免暴露后台功能地图/接口清单)。
//
// 逻辑:读 admin_gate cookie(= 调用方 access token 的镜像)并校验。
//   - 有效  → 200(Traefik 放行,nginx 下发 SPA);
//   - 缺失/无效/过期 → 302 到 /gate/(登录壳)。forwardAuth 会把这个 302 原样透传给浏览器,
//     于是陌生人看到的是登录壳,而不是 401 白屏,也永远拿不到大包。
//
// 本端点**不挂 JWT 中间件**(它自己就是那道校验),且失败一律「拒绝(跳壳)」而非 500 ——
// 宁可把人挡在壳外,也不能因为内部错误把门敞开。
//
// user 侧说明:token 校验走 user 自己注册的 aurora JWTService(与后台域共享 JWT_SECRET,aurora
// 只验签名不验 issuer),因此 admin 或 user 任一方签发的 token 都能在此通过 —— 相位 A 的「备用」
// 端点,forwardAuth 尚未指向它,真正翻转在相位 B(纯改 values adminGate.verifyAddress)。
func GateVerify(c *contracts.RequestContext) (interface{}, bizerr.BizError) {
	token, err := c.Cookie(gateCookieName)
	if err != nil || token == "" {
		redirectToGateShell(c)
		return nil, nil
	}

	var jwtService auroraFeature.JWTService
	if ferr := c.App.Find(&jwtService); ferr != nil {
		// 兜底:拿不到校验器就当没通过(跳壳),绝不放行。
		redirectToGateShell(c)
		return nil, nil
	}

	if _, verr := jwtService.ValidateToken(token); verr != nil {
		redirectToGateShell(c)
		return nil, nil
	}

	// 通过:任意 2xx 即让 Traefik 放行(body 会被 forwardAuth 忽略)。
	return gin.H{"ok": true}, nil
}

// redirectToGateShell 写一个 302 到登录壳。
//
// ⚠️ 必须回**外部绝对 URL**,不能回相对 /gate/。因为 verify 是 Traefik forwardAuth 的**内部子请求**,
// 它看到的 Host 是集群内部地址(user.<ns>.svc.cluster.local:8083);若回相对路径,Traefik 会把它按
// 内部地址解析成 http://user.<ns>.svc...:8083/gate/ 再透传给浏览器 → 浏览器解析内部域名失败(NXDOMAIN)。
// forwardAuth 会把原始外部请求的 scheme/host 放在 X-Forwarded-Proto / X-Forwarded-Host,据此拼外部绝对地址。
func redirectToGateShell(c *contracts.RequestContext) {
	// 带上用户原本要访问的路径(forwardAuth 的 X-Forwarded-Uri):登录壳静默续期 / 登录成功后据此跳回,
	// 不再一律落首页。只收「同源绝对路径」防开放重定向,且排除门禁/登录自身防死循环。
	returnQuery := ""
	if uri := c.GetHeader("X-Forwarded-Uri"); isSafeReturnPath(uri) {
		returnQuery = "?returnUrl=" + url.QueryEscape(uri)
	}

	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		// 兜底:拿不到外部 host 就回相对路径(至少不把内部地址暴露/发给浏览器)。
		c.Redirect(http.StatusFound, gateShellPath+returnQuery)
		return
	}
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "https"
	}
	c.Redirect(http.StatusFound, scheme+"://"+host+gateShellPath+returnQuery)
}

// safeReturnCharset 站内路径允许的字符集(RFC3986 常用子集):字母数字 + 路径/查询常见符号。
// **不含反斜杠、空白、控制符、尖括号引号** —— 反斜杠会被浏览器归一成 '/',`/\evil.com` → `//evil.com`
// → 开放重定向,必须连字符集一起拦死(RE2 无 lookahead,故 '//' 单独用 HasPrefix 判)。
var safeReturnCharset = regexp.MustCompile(`^[A-Za-z0-9\-._~/?#=&@%+:,]+$`)

// isSafeReturnPath 只认「站内绝对路径」,防开放重定向。要求:
//   - 非空、≤2048;单个 '/' 开头(排除 '//'、'/\' 之类协议相对/反斜杠绕过);
//   - 不含反斜杠;整串只由安全字符集组成;
//   - 不是门禁壳 / 登录页自身(避免跳回来又被挡 → 死循环)。
//
// 带 query 的深链(/customer?email=a@b.com)照常放行。这是第一道;门禁壳里还用 new URL 判同源兜底。
func isSafeReturnPath(p string) bool {
	if p == "" || len(p) > 2048 {
		return false
	}
	if p[0] != '/' || strings.HasPrefix(p, "//") || strings.Contains(p, `\`) {
		return false
	}
	if strings.HasPrefix(p, gateShellPath) || strings.HasPrefix(p, "/login") {
		return false
	}
	return safeReturnCharset.MatchString(p)
}
