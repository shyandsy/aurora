package loginguard

// policy.go —— loginguard 的**策略层**(阈值 + 来源),与 guard.go 的执行机制解耦。
// loginguard 只认这里的 LoginPolicy / LoginPolicyProvider,不认任何产品的 SecurityConfig(依赖倒置)。

// LoginPolicy 是 loginguard 的阈值策略。各字段 <=0 的语义见注释(用零值关闭对应维度,不由框架塞默认)。
type LoginPolicy struct {
	// IPFailLimit 窗口内同一 IP 允许的最大失败次数;超过即锁该 IP。<=0 关闭「IP 失败锁」。
	IPFailLimit int
	// IPWindowSeconds IP/账号 失败计数的滑动窗口秒数。IP/账号失败锁启用时必须 >0。
	IPWindowSeconds int
	// IPLockSeconds IP 失败锁的时长秒数。IPFailLimit>0 时必须 >0。
	IPLockSeconds int
	// IPPerHour 同一 IP 每小时允许的成功登录数上限(挡"撞库成功后批量登录")。<=0 关闭。
	IPPerHour int

	// AcctFailLimit 窗口内同一账号(跨 IP 汇总,挡分布式撞库)允许的最大失败次数。<=0 关闭「账号维度」。
	AcctFailLimit int
	// AcctLockSeconds 账号锁时长秒数,决定账号维度的**模式**:
	//   - >0:硬锁 —— 失败超 AcctFailLimit 即锁该账号,预检拦截(deploy 管理台用:锁了运维去 Redis 清)。
	//   - =0:只计数 —— 照常累计(供「还剩几次」提示 / 监控),但**从不上锁**、预检从不拦
	//         (homeserver 面向公网用:账号硬锁会被拿来锁死他人账号 = DoS)。
	AcctLockSeconds int
}

// DefaultPolicy 一套**安全的**起步阈值:账号维度默认 only-count(AcctLockSeconds=0),要硬锁的产品
// (如 deploy 管理台)显式覆盖 AcctLockSeconds。阈值仍是产品策略,这里只是省去从零填的样板。
func DefaultPolicy() LoginPolicy {
	return LoginPolicy{
		IPFailLimit:     5,
		IPWindowSeconds: 300,
		IPLockSeconds:   900,
		IPPerHour:       60,
		AcctFailLimit:   10,
		AcctLockSeconds: 0, // 默认只计数不硬锁(安全默认);要硬锁的产品显式设 >0
	}
}

// Valid 报告策略是否自洽:开了某个锁/上限就必须配齐其窗口/时长。用于 feature 启动时校验初始快照。
func (p LoginPolicy) Valid() bool {
	if p.IPFailLimit > 0 && (p.IPWindowSeconds <= 0 || p.IPLockSeconds <= 0) {
		return false
	}
	if p.AcctFailLimit > 0 && p.IPWindowSeconds <= 0 { // 账号计数复用同一窗口
		return false
	}
	if p.AcctLockSeconds < 0 {
		return false
	}
	return true
}

// LoginPolicyProvider 是策略来源(依赖倒置:loginguard 只认这个接口,不认任何产品的 SecurityConfig)。
// 每次判定都会调用它 —— 从而支持运行时改阈值(provider 内部应内存缓存,**绝不在此查 DB**)。
type LoginPolicyProvider interface {
	LoginPolicy() LoginPolicy
}

// StaticPolicy 把一份固定策略封成 provider(阈值编译期定、不在线调的产品用)。
func StaticPolicy(p LoginPolicy) LoginPolicyProvider { return staticProvider{p} }

type staticProvider struct{ p LoginPolicy }

func (s staticProvider) LoginPolicy() LoginPolicy { return s.p }
