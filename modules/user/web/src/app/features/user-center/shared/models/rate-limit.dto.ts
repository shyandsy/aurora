/**
 * 登录限流 / 被锁登录(loginguard, user namespace)后台管理的 DTO。
 *
 * 契约来源:services/user 的 `GET /rate-limit/locked`(查看=user.get)与
 * `POST /rate-limit/locked/unlock`(解锁=user.update)。后端刻意复用用户权限,不新增 feature。
 */

/** 被锁维度:ip → key 是 IP;account → key 是邮箱。 */
export type LockedScope = 'ip' | 'account';

/**
 * 命中原因(来自 loginguard 预检):
 *  - ip_locked   IP 因失败次数过多被锁;
 *  - ip_hour_cap IP 触发每小时上限;
 *  - acct_locked 账号因失败次数过多被锁。
 * 用 string 而非联合类型收窄,保证后端将来新增取值时前端不解码失败(未知值原样展示)。
 */
export type LockedReason = 'ip_locked' | 'ip_hour_cap' | 'acct_locked';

/** 单条被锁项。 */
export interface LockedLoginItem {
  /** 维度:ip / account。 */
  scope: LockedScope;
  /** scope=ip → IP;scope=account → 邮箱。 */
  key: string;
  /** 最近相关邮箱(可能为空)。 */
  email: string;
  /** 命中原因,取值见 LockedReason;未知值前端原样展示。 */
  reason: string;
  /** 距自动解除还剩秒数(<=0 表示即将/已自动解除)。 */
  retryAfterSeconds: number;
  /** 记录时刻,unix 秒。 */
  updatedAt: number;
}

/** GET /rate-limit/locked 响应。 */
export interface ListLockedResponse {
  items: LockedLoginItem[];
}

/** POST /rate-limit/locked/unlock 请求体。 */
export interface UnlockLockedRequest {
  scope: LockedScope;
  key: string;
}

/** POST /rate-limit/locked/unlock 响应。 */
export interface UnlockLockedResponse {
  success: boolean;
}
