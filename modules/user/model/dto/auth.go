package dto

// LoginReq 登录请求
type LoginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResp 登录响应。
// 开了两步验证的用户:第一步密码验过后 TwoFactorRequired=true + PendingToken,其余字段留空;
// 前端据此弹验证码框,凭 PendingToken + 码调 /auth/login/2fa 换正式 token。
type LoginResp struct {
	AccessToken     string   `json:"accessToken"`
	TokenType       string   `json:"tokenType"`
	ExpiresInSecond int64    `json:"expiresInSecond"`
	RefreshToken    string   `json:"refreshToken"`
	Features        []string `json:"features"`
	User            *User    `json:"user"`

	TwoFactorRequired bool   `json:"twoFactorRequired,omitempty"`
	PendingToken      string `json:"pendingToken,omitempty"`

	// MustEnrollTwoFactor=true:强制两步验证但该账号尚未绑定。此时 AccessToken 是**空权限的仅绑定 token**
	// (过不了任何带权限接口,只能调 2FA 绑定端点),前端须强制引导绑定,绑定完(confirm 返回正式 token)才能操作。
	MustEnrollTwoFactor bool `json:"mustEnrollTwoFactor,omitempty"`
}

// ---- 两步验证(TOTP,Google Authenticator)----

// TotpSetupReq 开始绑定(需已登录 + 重验密码)。
type TotpSetupReq struct {
	Password string `json:"password" binding:"required"`
}

// TotpSetupResp 返回 otpauth URI(前端渲染二维码)+ base32 密钥(手动录入)。
type TotpSetupResp struct {
	OtpauthURI string `json:"otpauthUri"`
	Secret     string `json:"secret"`
}

// TotpConfirmReq 确认绑定:输 app 上的 6 位码。
type TotpConfirmReq struct {
	Code string `json:"code" binding:"required"`
}

// TotpConfirmResp 确认绑定成功:一次性返回备用码(仅此一次,提示保存)+ 正式登录态。
// Login 让"强制绑定"场景绑定后直接进系统、无需重登(空权限的仅绑定 token 换成正式 token)。
type TotpConfirmResp struct {
	RecoveryCodes []string   `json:"recoveryCodes"`
	Login         *LoginResp `json:"login,omitempty"`
}

// TotpDisableReq 关闭两步验证:需密码 + 一个当前码。
type TotpDisableReq struct {
	Password string `json:"password" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

// Login2FAReq 登录第二步:凭 pendingToken + 6 位码(或备用码)换正式 token。
type Login2FAReq struct {
	PendingToken string `json:"pendingToken" binding:"required"`
	Code         string `json:"code" binding:"required"`
}

// LogoutReq 登出请求（accessToken 从 Authorization 头取，refreshToken 从 body 取；两者都会被拉黑）
type LogoutReq struct {
	RefreshToken string `json:"refreshToken"`
}

// RefreshReq 刷新 token 请求（用登录时下发的 refreshToken 换新的 accessToken，无需已登录的 accessToken）
type RefreshReq struct {
	RefreshToken string `json:"refreshToken"`
}

// RefreshResp 刷新 token 响应（access + refresh 都会轮换）
type RefreshResp struct {
	AccessToken     string `json:"accessToken"`
	TokenType       string `json:"tokenType"`
	ExpiresInSecond int64  `json:"expiresInSecond"`
	RefreshToken    string `json:"refreshToken"`
}
