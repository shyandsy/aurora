import { Injectable } from '@angular/core';
import { environment } from '../../../environments/environment';

/**
 * Storage service for managing localStorage
 */
@Injectable({
  providedIn: 'root'
})
export class StorageService {
  private readonly TOKEN_KEY = 'admin_access_token';
  private readonly REFRESH_TOKEN_KEY = 'admin_refresh_token';
  private readonly USER_KEY = 'admin_user';
  // 门禁标志:强制两步验证但账号未绑定时置位。置位期间只发了「仅绑定 token」,
  // 路由守卫据此把任何业务路由都拦回绑定页,绑定完成后清除。
  private readonly ENROLLMENT_PENDING_KEY = 'admin_2fa_enrollment_pending';

  // 过期跳登录/门禁前暂存「用户本要去的 URL」;整页重载或登录成功后据此跳回,避免一律回首页丢 returnUrl。
  // 刻意**不**在 clear() 里删它:forceLogin 会先 clear 再整页跳门禁,门禁往返(SPA 全新加载)后要靠它把人送回原处。
  private readonly RETURN_URL_KEY = 'admin_return_url';
  // returnUrl 时效:因为不进 clear(),若用户中途放弃(存了却没走完登录/门禁),它会残留在 localStorage。
  // 加 10 分钟窗口,超时即作废,避免「隔天重开被拽回昨天那个页」这类陈旧跳转。存的是 `${ts}|${url}`。
  private readonly RETURN_URL_TTL_MS = 10 * 60 * 1000;

  // 门禁 cookie 名。它只是 access token 的一份镜像:Traefik forwardAuth 只能看 cookie、
  // 看不到我们放 localStorage 里、用 Authorization 头发的 token,所以镜像一份给它做「是否下发 SPA」的判定。
  // 见 user 模块 controller/auth/gate.go 的 GateVerify。SPA 在**每次写/清 access token 时同步**这个 cookie,
  // 这样登录、静默续期(拦截器)后 cookie 始终等于当前 token,整页重载能通过门禁。
  //
  // **必须与后端 user.Config.GateCookie 同名**,故走 environment 注入(单一事实来源),不硬编码——
  // 否则项目改了后端 GateCookie、前端还写旧名 → 名字不一致、门禁失效(和 RateLimitNamespace 同类陷阱)。
  private readonly GATE_COOKIE = environment.gateCookie;

  // 会话 cookie(不设 Max-Age):关浏览器即清;重开时靠 refresh token 静默续期重建。
  // Secure + SameSite=Strict:只随 https 同站请求发出(localhost 也算安全上下文,dev 不受影响)。
  private setGateCookie(token: string): void {
    document.cookie = `${this.GATE_COOKIE}=${token}; Path=/; SameSite=Strict; Secure`;
  }
  private clearGateCookie(): void {
    document.cookie = `${this.GATE_COOKIE}=; Path=/; Max-Age=0; SameSite=Strict; Secure`;
  }

  /**
   * Save access token
   */
  setAccessToken(token: string): void {
    localStorage.setItem(this.TOKEN_KEY, token);
    this.setGateCookie(token); // 同步门禁 cookie,保证整页重载能过 forwardAuth
  }

  /**
   * Get access token
   */
  getAccessToken(): string | null {
    return localStorage.getItem(this.TOKEN_KEY);
  }

  /**
   * Save refresh token
   */
  setRefreshToken(token: string): void {
    localStorage.setItem(this.REFRESH_TOKEN_KEY, token);
  }

  /**
   * Get refresh token
   */
  getRefreshToken(): string | null {
    return localStorage.getItem(this.REFRESH_TOKEN_KEY);
  }

  /**
   * Save user data
   */
  setUser(user: any): void {
    localStorage.setItem(this.USER_KEY, JSON.stringify(user));
  }

  /**
   * Get user data
   */
  getUser(): any | null {
    const userStr = localStorage.getItem(this.USER_KEY);
    return userStr ? JSON.parse(userStr) : null;
  }

  /**
   * 置位/清除「未绑定两步验证」门禁标志。
   */
  setEnrollmentPending(pending: boolean): void {
    if (pending) {
      localStorage.setItem(this.ENROLLMENT_PENDING_KEY, '1');
    } else {
      localStorage.removeItem(this.ENROLLMENT_PENDING_KEY);
    }
  }

  /**
   * 是否处于「未绑定两步验证」门禁态(仅绑定 token,不能操作任何业务)。
   */
  isEnrollmentPending(): boolean {
    return localStorage.getItem(this.ENROLLMENT_PENDING_KEY) === '1';
  }

  /**
   * Clear all stored data
   */
  clear(): void {
    localStorage.removeItem(this.TOKEN_KEY);
    localStorage.removeItem(this.REFRESH_TOKEN_KEY);
    localStorage.removeItem(this.USER_KEY);
    localStorage.removeItem(this.ENROLLMENT_PENDING_KEY);
    this.clearGateCookie(); // 清凭据的同时关闭门禁 cookie,登出/续期失败后整页重载会被挡回登录壳
  }

  /**
   * 暂存「过期跳登录前用户本要去的 URL」。登录/门禁往返后由 App 启动或登录页取出跳回。
   * 跳过登录/根/门禁/绑定这些「不该跳回」的目标,避免回环。
   */
  saveReturnUrl(url: string): void {
    if (!url || url === '/' || url.startsWith('/login') || url.startsWith('/gate') || url.startsWith('/two-factor')) {
      return;
    }
    localStorage.setItem(this.RETURN_URL_KEY, `${Date.now()}|${url}`);
  }

  /**
   * 取出并清除暂存的 returnUrl(一次性)。无、格式异常或已超过 RETURN_URL_TTL_MS 均返回 null。
   */
  takeReturnUrl(): string | null {
    const raw = localStorage.getItem(this.RETURN_URL_KEY);
    if (!raw) {
      return null;
    }
    localStorage.removeItem(this.RETURN_URL_KEY); // 一次性:无论用不用得上,取出即清
    const sep = raw.indexOf('|');
    if (sep <= 0) {
      return null; // 非预期格式(老数据/被外部写脏),丢弃
    }
    const ts = Number(raw.slice(0, sep));
    if (!Number.isFinite(ts) || Date.now() - ts > this.RETURN_URL_TTL_MS) {
      return null; // 过期作废,避免陈旧跳转
    }
    return raw.slice(sep + 1) || null;
  }

  /**
   * Check if user is authenticated
   */
  isAuthenticated(): boolean {
    return !!this.getAccessToken();
  }
}

