/**
 * Authentication DTOs
 */

export interface LoginRequest {
  email: string;
  password: string;
}

export interface Role {
  id: number;
  name: string;
  created: string;
  modified: string;
}

export interface User {
  id: number;
  email: string;
  roleId: number;
  status: number;
  created: string;
  modified: string;
  role?: Role;
  features: string[];
  // 后端据实回显该账号是否已绑定两步验证(TOTP)。设置页据此显示启用/关闭。
  twoFactorEnabled?: boolean;
}

export interface LoginResponse {
  accessToken: string;
  tokenType: string;
  expiresInSecond: number;
  refreshToken: string;
  features: string[];
  user: User;

  // 该账号已绑定 2FA:需第二步验证码。此时正式字段留空,凭 pendingToken + 码调 /auth/login/2fa 换正式 token。
  twoFactorRequired?: boolean;
  pendingToken?: string;

  // 强制两步验证但该账号尚未绑定:accessToken 是**空权限的仅绑定 token**(只能调 2fa setup/confirm),
  // 前端须置门禁标志、强制引导绑定,绑定完(confirm 返回正式 login)才放行进系统。
  mustEnrollTwoFactor?: boolean;
}

// 登录第二步:pendingToken + 6 位 TOTP 码(备用码也走这个字段)。
export interface LoginTwoFactorRequest {
  pendingToken: string;
  code: string;
}

// 开始绑定(需已登录 + 重验密码)。
export interface TotpSetupRequest {
  password: string;
}

// 返回 otpauth URI(前端渲染二维码)+ base32 密钥(供手动录入)。
export interface TotpSetupResponse {
  otpauthUri: string;
  secret: string;
}

// 确认绑定:输 app 上的 6 位码。
export interface TotpConfirmRequest {
  code: string;
}

// 确认绑定成功:一次性返回备用码(仅此一次,强提示保存)+ 正式登录态。
export interface TotpConfirmResponse {
  recoveryCodes: string[];
  login?: LoginResponse;
}

// 关闭两步验证:需密码 + 一个当前码。
export interface TotpDisableRequest {
  password: string;
  code: string;
}

// 全局强制开关:GET/PUT /setting/require_two_factor。
export interface RequireTwoFactorSetting {
  require: boolean;
}

