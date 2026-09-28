package user

// Package usercenter 是用户中心(user 服务)的「可挂载模块」装配层。
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
	// TOTPKeyEnv 是 TOTP 密钥等敏感凭据(AES-256-GCM,common/secret)加解密密钥的**环境变量名**。
	// 原先硬编码为 "USER_GOOGLE_TOTP_AUTH_KEY",分布在 cmd/main.go(启动校验)与
	// service/user/credential.go(运行时加解密)两处。收进 Config 后成为单一事实来源:
	// Setup 会把它同时喂给启动校验与运行时(service/user.SetCredentialKeyEnv)。
	// 留空 → "USER_GOOGLE_TOTP_AUTH_KEY"。
	TOTPKeyEnv string

	// GateCookie 是后台「下载门禁」forwardAuth 读取的 cookie 名(GateVerify 用)。
	// 原先硬编码为 "admin_gate"(controller/auth/gate.go)。前端 storage.service 与登录壳写同一个名字,
	// 故它对每个接入项目是可配置的对外约定。留空 → "admin_gate"。
	GateCookie string

	// TablePrefix 是 goose 迁移版本表的前缀(homeserver 用 "user_",版本表即 user_goose_db_version)。
	//
	// ⚠️ 声明字段,当前**不由本模块注入**:迁移在 bootstrap.InitDefaultApp() 内执行,发生在
	// AddFeature(本模块 Setup)之前,故前缀只能由部署期环境变量 GOOSE_TABLE_PREFIX 提供,
	// facade 无从在 Setup 阶段左右它(那时迁移已跑完)。此字段用于把「本项目用什么前缀」写进模块配置契约、
	// 供接入方在 InitDefaultApp 之前自行 export;真正下沉到模块需连同 App 初始化一起搬,属后续步骤。
	TablePrefix string

	// RateLimitNamespace 是登录暴力破解防护(aurora loginguard)引擎的 namespace,做 Redis key 前缀
	// (rate_limit:<ns>:login_*)。原先硬编码 "user"。留空 → "user"。
	//
	// ⚠️ 注意:service/ratelimit 的「被锁列表」索引 key(rate_limit:user:index[:account])目前仍是
	// 硬编码常量,与本 namespace 耦合但**未**走 Config;改 namespace 需同步改那两个常量,否则后台列表读不到。
	// 详见报告的「未干净收进 Config」清单。
	RateLimitNamespace string
}

const (
	defaultTOTPKeyEnv         = "USER_GOOGLE_TOTP_AUTH_KEY"
	defaultGateCookie         = "admin_gate"
	defaultRateLimitNamespace = "user"
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
	if c.RateLimitNamespace == "" {
		c.RateLimitNamespace = defaultRateLimitNamespace
	}
	return c
}
