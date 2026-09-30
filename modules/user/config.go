package user

import "github.com/shyandsy/aurora/feature/loginguard"

// Package user 是用户中心(user 服务)的「可挂载模块」装配层。
//
// 它把原先散在 cmd/main.go + cmd/providers.go + controller/routes.go 里的
// 「注册哪些 Feature / Datalayer / Service、启动校验、路由表」全部收拢到一个自包含入口,
// 对外只暴露三样东西:
//   - Config              —— 每个接入项目会不一样的配置(env 名、cookie 名、namespace…)
//   - NewFeature —— 返回一个 contracts.Features,Setup 里做全部注册 + 启动校验
//   - Routes               —— 返回用户中心的全部路由(路径 / handler / 中间件 / 权限点逐字不变)
//
// 目的:在真实代码上把「共享代码」与「每项目配置」钉清楚。将来本模块会整体物理搬到独立仓,
// 因此这一层刻意写成自包含:除 Config 外不新增对宿主(homeserver)特有包的依赖,
// 它依赖的都是本模块自身的 sub-package(controller/service/datalayer/model)与 aurora / common。

// Config 收拢用户中心里「每个接入项目可能不一样」的配置项。
// 宿主(homeserver)在挂载时按当前值填入,行为与内联时逐字一致。
// 所有字段留空时,各自回落到 homeserver 现行默认值(见 withDefaults),保证零行为变化。
type Config struct {
	// TOTPKeyEnv 是 TOTP 密钥等敏感凭据(AES-256-GCM,encryption 包)加解密密钥的**环境变量名**。
	// 原先硬编码为 "USER_GOOGLE_TOTP_AUTH_KEY",分布在 cmd/main.go(启动校验)与
	// service/user/credential.go(运行时加解密)两处。收进 Config 后成为单一事实来源:
	// Setup 会把它同时喂给启动校验与运行时(service/user.SetCredentialKeyEnv)。
	// 留空 → "USER_GOOGLE_TOTP_AUTH_KEY"。
	TOTPKeyEnv string

	// GateCookie 是后台「下载门禁」forwardAuth 读取的 cookie 名(GateVerify 用)。
	// 原先硬编码为 "admin_gate"(controller/auth/gate.go)。前端 storage.service 与登录壳写同一个名字,
	// 故它对每个接入项目是可配置的对外约定。留空 → "admin_gate"。
	GateCookie string

	// 关于 goose 迁移版本表前缀:**不在本 Config 里**。迁移在 bootstrap.InitDefaultApp() 内、
	// 早于模块 Setup 就跑完了,Config 够不着;且 GOOSE_TABLE_PREFIX 只前缀**版本表**
	// (user_goose_db_version),**从不影响业务表**——业务表名写死在迁移 SQL 里(user_*)。
	// 故版本表隔离是**部署期环境变量 GOOSE_TABLE_PREFIX** 的事(各服务共库时隔离各自 goose 版本流),
	// 与本模块配置无关,不设字段以免误导"设了能给业务表加前缀"。

	// 关于登录限流 namespace:**不在本 Config 里**。限流分区键语义上就是「哪个服务」,归公共组件
	// aurora ratelimit/loginguard 拥有——留空则**自动取 SERVICE_NAME**(按服务天然解耦)。本模块只声明
	// 登录桶,不决定 namespace;后台「被锁列表」索引也从 loginguard.Guard.Namespace() 取同一前缀,单一源。

	// LoginPolicyProvider 可选:注入登录限流阈值来源,让阈值**运行时可调**(改设置即生效,不改代码/不重部署)。
	//   - 留空 → 内置 StaticPolicy(编译期硬锁:DefaultPolicy + AcctLockSeconds=900),即现行行为,零回归;
	//   - 传值 → 用宿主自己的实现(如从设置表读、内存缓存的 provider;deploy 的 DBProvider 读 deploy_setting)。
	// provider 只决定「阈值从哪来」;账号维度硬锁 vs 只计数仍由它返回的 LoginPolicy.AcctLockSeconds 编码
	// (>0 硬锁 / =0 只计数)。loginguard feature 启动时会校验初始快照自洽(不自洽即 fail-startup)。
	LoginPolicyProvider loginguard.LoginPolicyProvider
}

const (
	defaultTOTPKeyEnv = "USER_GOOGLE_TOTP_AUTH_KEY"
	defaultGateCookie = "admin_gate"
)

// withDefaults 返回把空字段回落到 homeserver 现行默认值后的副本。
// 这保证「挂载时不传任何值」与「内联旧代码」行为逐字一致。
func (c Config) withDefaults() Config {
	if c.TOTPKeyEnv == "" {
		c.TOTPKeyEnv = defaultTOTPKeyEnv
	}
	if c.GateCookie == "" {
		c.GateCookie = defaultGateCookie
	}
	return c
}

// defaultAcctLockSeconds 是内置(未注入 provider 时)的账号硬锁时长:失败超阈值即锁账号、预检拦截。
const defaultAcctLockSeconds = 900

// resolveLoginPolicyProvider 决定登录限流阈值来源:
//   - Config.LoginPolicyProvider 非空 → 用宿主注入的(运行时可调,如从设置表读);
//   - 留空 → 内置 StaticPolicy(DefaultPolicy + AcctLockSeconds=900,编译期硬锁),即现行行为、零回归。
func resolveLoginPolicyProvider(c Config) loginguard.LoginPolicyProvider {
	if c.LoginPolicyProvider != nil {
		return c.LoginPolicyProvider
	}
	p := loginguard.DefaultPolicy()
	p.AcctLockSeconds = defaultAcctLockSeconds
	return loginguard.StaticPolicy(p)
}
