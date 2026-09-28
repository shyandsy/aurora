package user

import (
	"context"
	"os"

	"github.com/shyandsy/aurora/config"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/feature/geoip"
	"github.com/shyandsy/aurora/feature/loginguard"
	"github.com/shyandsy/aurora/feature/tokenguard"
	"github.com/shyandsy/aurora/logger"

	"github.com/shyandsy/aurora/modules/user/controller/auth"
	"github.com/shyandsy/aurora/modules/user/datalayer"
	serviceFeature "github.com/shyandsy/aurora/modules/user/service/feature"
	serviceMicroserviceToken "github.com/shyandsy/aurora/modules/user/service/microservice_token"
	serviceMicroserviceTokenFeature "github.com/shyandsy/aurora/modules/user/service/microservice_token_feature"
	serviceRateLimit "github.com/shyandsy/aurora/modules/user/service/ratelimit"
	serviceRole "github.com/shyandsy/aurora/modules/user/service/role"
	serviceRoleFeature "github.com/shyandsy/aurora/modules/user/service/role_feature"
	"github.com/shyandsy/aurora/modules/user/service/rolefeaturepublisher"
	serviceUser "github.com/shyandsy/aurora/modules/user/service/user"
)

// userCenterFeature 是用户中心的可挂载 Feature(实现 contracts.Features)。
// New 只存配置,全部注册与启动校验都在 Setup 里做(aurora 的 AddFeature 会同步调用 Setup)。
type userCenterFeature struct {
	cfg Config
}

// NewFeature 构造用户中心模块。cfg 的空字段回落到 homeserver 现行默认值,
// 用当前值挂载即与内联旧代码逐字等价。
func NewFeature(cfg Config) contracts.Features {
	return &userCenterFeature{cfg: cfg.withDefaults()}
}

func (f *userCenterFeature) Name() string { return "usercenter" }

func (f *userCenterFeature) Close() error { return nil }

// Setup 承接原 cmd/main.go 的启动校验 + cmd/providers.go:registerProviders 的全部注册 +
// 启动期读模型重建。顺序、依赖关系与原实现逐一保持。
//
// 关于时序:aurora 的 app.AddFeature 会**同步**调用被加入 Feature 的 Setup;因此本 Setup 内再
// AddFeature(geoip / loginguard / tokenguard)会就地、按序完成它们各自的 Setup,与原先在
// registerProviders 里顺序 AddFeature 完全一致。
func (f *userCenterFeature) Setup(app contracts.App) error {
	// —— 0. 参数化项落到各 sub-package(在任何注册 / 请求之前)——
	// TOTP 凭据密钥 env 名:同时作用于下面的启动校验与运行时加解密(单一事实来源)。
	serviceUser.SetCredentialKeyEnv(f.cfg.TOTPKeyEnv)
	// 门禁 cookie 名:GateVerify 读取。
	auth.SetGateCookieName(f.cfg.GateCookie)

	// —— 1. 凭据密钥强制校验(原 main.go)——
	// 未配置 / 无效则拒绝启动,绝不回落内置占位密钥(占位密钥人人可知,落库密文形同明文)。
	// 共享同一份凭据的多个服务必须配置同一把密钥。保持原有 FATAL 日志 + os.Exit(1) 语义。
	if err := serviceUser.ValidateCredentialKey(); err != nil {
		logger.Errorf("[FATAL] 凭据加密密钥无效: %v。生产/本地/CI 均须配置 %s(base64 32 字节,或任意口令)。", err, f.cfg.TOTPKeyEnv)
		os.Exit(1)
	}

	// —— 2. 注册 providers(Feature / Datalayer / Service),顺序与依赖关系逐一保持 ——
	f.registerProviders(app)

	// —— 3. 启动时全量重建 role→feature 读模型(best-effort,失败不阻断启动)——
	rebuildRoleFeatureReadModel(app)

	return nil
}

// registerProviders 注册所有依赖注入的 providers。
// 顺序:先装配 Feature(geoip),再注册被依赖的 Datalayer,最后注册依赖它们的 Service。
// (逐字搬自原 cmd/providers.go:registerProviders;仅把 homeserver 特定常量替换为 f.cfg 的值。)
func (f *userCenterFeature) registerProviders(app contracts.App) {
	// IP→归属地解析:库随 aurora feature/geoip 内嵌(go:embed),零配置、零外部文件。
	// 会话/设备清单据此把登录 IP 标注归属地。必须先于注入 geoip.Resolver 的 Service 装配。
	app.AddFeature(geoip.NewFeature(geoip.WithChinaFallback(true)))

	// 登录暴力破解防护(防护三件套之「登录前」):按 IP + 账号做失败计数 / 短期锁定 / 成功清理。
	// 本服务(内部用户中心)用**硬锁**模式 —— DefaultPolicy 基础上把 AcctLockSeconds 设 >0(900s):
	// 账号失败超阈值即锁账号、预检拦截,运维去 Redis 清(account 传明文 email 便于手动解锁)。
	// 阈值编译期定,故用 StaticPolicy;须在 redis feature(bootstrap 已装)之后、注入 Guard 的 Service 之前。
	// 计数机制建在 aurora ratelimit 引擎上;namespace(默认 "user")给引擎做 key 前缀 rate_limit:<ns>:...。
	userLoginPolicy := loginguard.DefaultPolicy()
	userLoginPolicy.AcctLockSeconds = 900
	app.ProvideAs(loginguard.StaticPolicy(userLoginPolicy), (*loginguard.LoginPolicyProvider)(nil))
	app.AddFeature(loginguard.NewLoginGuardFeature(f.cfg.RateLimitNamespace))

	// token 会话有效性内核(撤销 jti 黑名单 + 登录 IP 绑定):opt-in aurora feature,须在 redis + jwt 之后
	// (二者由 bootstrap.InitDefaultApp 已装配)。denyTTL 取 jwt refresh 寿命——它是「只有 jti、拿不到 token
	// 精确到期」时(Revoke(jti) 兜底路径,会话表撤销走它)黑名单条目的存活上界,须 >= 本服务最长 token 寿命,
	// 否则被撤 token 会在 TTL 到点后"复活"。被 UserService inject(tokenguard.Guard),须在其 ProvideAs 之前注册。
	var jc config.JWTConfig
	_ = config.ResolveConfig(&jc)
	app.AddFeature(tokenguard.NewTokenGuardFeature(jc.RefreshExpireOrDefault()))

	// 用户中心 Datalayers(user_users / user_roles / user_features / user_role_features)。
	app.ProvideAs(datalayer.NewUserDatalayer(app), (*datalayer.UserDatalayer)(nil))
	app.ProvideAs(datalayer.NewRoleDatalayer(app), (*datalayer.RoleDatalayer)(nil))
	app.ProvideAs(datalayer.NewFeatureDatalayer(app), (*datalayer.FeatureDatalayer)(nil))
	app.ProvideAs(datalayer.NewRoleFeatureDatalayer(app), (*datalayer.RoleFeatureDatalayer)(nil))
	// 登录会话/设备清单 Datalayer(user_session):列出 / 撤销 / 续期滚动。
	app.ProvideAs(datalayer.NewUserSessionDatalayer(app), (*datalayer.UserSessionDatalayer)(nil))
	// 微服务 token 管理 Datalayers(user_microservice_token_features / *_token):
	// 通用认证服务对外签发「服务间调用」所需的长效 access token,并记录/启停。
	app.ProvideAs(datalayer.NewMicroserviceTokenFeatureDatalayer(app), (*datalayer.MicroserviceTokenFeatureDatalayer)(nil))
	app.ProvideAs(datalayer.NewMicroserviceTokenFeatureTokenDatalayer(app), (*datalayer.MicroserviceTokenFeatureTokenDatalayer)(nil))

	// role→feature 读模型写侧发布器(写穿共享 Redis 读模型,供双模中间件按 role 展开判权)。
	// 依赖 RedisService(aurora 内建);被 RoleService / RoleFeatureService inject,须在其之前注册。
	// 返回接口(可空),满足 ProvideAs 可空要求。
	app.ProvideAs(rolefeaturepublisher.NewPublisher(app), (*rolefeaturepublisher.Publisher)(nil))

	// 用户中心 Services(登录签发 JWT + 用户/角色/权限管理)。
	// UserService 依赖 User/Role/Feature/Session Datalayer + aurora JWTService(签发/校验/黑名单/refresh 均走它)。
	app.ProvideAs(serviceUser.NewUserService(app), (*serviceUser.UserService)(nil))
	app.ProvideAs(serviceRole.NewRoleService(app), (*serviceRole.RoleService)(nil))
	app.ProvideAs(serviceFeature.NewFeatureService(app), (*serviceFeature.FeatureService)(nil))
	app.ProvideAs(serviceRoleFeature.NewRoleFeatureService(app), (*serviceRoleFeature.RoleFeatureService)(nil))

	// 微服务 token 管理 Services:
	//   MicroserviceTokenService        —— 签发/查询/启停「服务间调用」token(签发走 aurora Claims + TokenType=access,
	//                                       用本服务同域 JWT_SECRET 本地签名,黑名单复用 aurora 导出的 key 前缀常量)。
	//   MicroserviceTokenFeatureService —— 预定义微服务身份 + feature 列表的增删改查。
	app.ProvideAs(serviceMicroserviceToken.NewMicroserviceTokenService(app), (*serviceMicroserviceToken.MicroserviceTokenService)(nil))
	app.ProvideAs(serviceMicroserviceTokenFeature.NewMicroserviceTokenFeatureService(app), (*serviceMicroserviceTokenFeature.MicroserviceTokenFeatureService)(nil))

	// 登录「被锁列表 + 解锁」后台服务:读 loginguard(user namespace)的锁 + user 专属索引,供后台管理。
	// 须在 loginguard feature(上面已 AddFeature)之后 —— 它注入 loginguard.Guard。
	app.ProvideAs(serviceRateLimit.NewLockAdminService(app), (*serviceRateLimit.LockAdminService)(nil))
}

// rebuildRoleFeatureReadModel 启动时全量重建 role→feature 读模型:遍历 user_roles,逐 role 展开 feature,
// 整键灌入共享 Redis(保证冷启动后读模型键常热,双模中间件对 role-in-token 可解析)。
// best-effort:读库/展开/写入失败只记日志、绝不阻断启动(本批发号仍 feature-in-token,读模型无消费者)。
// (逐字搬自原 cmd/main.go:rebuildRoleFeatureReadModel。)
func rebuildRoleFeatureReadModel(app contracts.App) {
	var roleDL datalayer.RoleDatalayer
	var featureDL datalayer.FeatureDatalayer
	var publisher rolefeaturepublisher.Publisher
	if err := app.Find(&roleDL); err != nil {
		logger.Errorf("[rolefeature] 启动重建:解析 RoleDatalayer 失败,跳过: %v", err)
		return
	}
	if err := app.Find(&featureDL); err != nil {
		logger.Errorf("[rolefeature] 启动重建:解析 FeatureDatalayer 失败,跳过: %v", err)
		return
	}
	if err := app.Find(&publisher); err != nil {
		logger.Errorf("[rolefeature] 启动重建:解析 Publisher 失败,跳过: %v", err)
		return
	}

	ctx := context.Background()
	roles, err := roleDL.GetAll(ctx)
	if err != nil {
		logger.Errorf("[rolefeature] 启动重建:读取全部 role 失败,跳过: %v", err)
		return
	}
	all := make(map[string][]string, len(roles))
	for i := range roles {
		names, err := featureDL.GetExpandedNamesByRoleID(ctx, roles[i].ID)
		if err != nil {
			logger.Errorf("[rolefeature] 启动重建:展开 role id=%d name=%q 失败,跳过该 role: %v", roles[i].ID, roles[i].Name, err)
			continue
		}
		all[roles[i].Name] = names
	}
	if err := publisher.RebuildAll(ctx, all); err != nil {
		logger.Errorf("[rolefeature] 启动重建:写入读模型失败: %v", err)
		return
	}
	logger.Infof("[rolefeature] 启动重建完成:已发布 %d 个 role 到读模型", len(all))
}
