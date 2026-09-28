package dto

// user 登录「被锁」维度。IP 与账号共用一份列表,前端据此区分展示与解锁。
const (
	UserLockScopeIP      = "ip"
	UserLockScopeAccount = "account"
)

// UserLockedEntry 一条「当前被锁」记录(IP 或账号维度)。
type UserLockedEntry struct {
	Scope string `json:"scope"` // "ip" 或 "account"
	// Key scope=ip 时是 IP;scope=account 时是明文 email。解锁按 scope+key 定位。
	Key string `json:"key"`
	// Email scope=ip 时是该 IP 最近一次尝试用的邮箱(供辨认,可能为空);scope=account 时同 Key。
	Email string `json:"email"`
	// Reason 命中原因:ip_locked / ip_hour_cap / acct_locked(来自 loginguard 预检)。
	Reason string `json:"reason"`
	// RetryAfterSeconds 距自动解除还剩几秒。
	RetryAfterSeconds int64 `json:"retryAfterSeconds"`
	// UpdatedAt 记录进索引的时刻(unix 秒),列表按它倒序。
	UpdatedAt int64 `json:"updatedAt"`
}

// UserLockedListResp 被锁列表响应。
type UserLockedListResp struct {
	Items []UserLockedEntry `json:"items"`
}

// UnlockReq 解锁请求,按维度+键定位。key 放 body(避免 IPv6 冒号 / 邮箱 @ 在路径出岔子)。
type UnlockReq struct {
	Scope string `json:"scope"`
	Key   string `json:"key"`
}
