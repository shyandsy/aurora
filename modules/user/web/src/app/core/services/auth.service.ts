import { Injectable, inject } from '@angular/core';
import { Router } from '@angular/router';
import { Observable } from 'rxjs';
import { StorageService } from './storage.service';
import { ApiService } from '../../shared/services/api.service';
import { environment } from '../../../environments/environment';
import { extractFeaturesFromToken } from '../../shared/utils/jwt.util';
import {
  LoginRequest,
  LoginResponse,
  LoginTwoFactorRequest,
  TotpSetupRequest,
  TotpSetupResponse,
  TotpConfirmRequest,
  TotpConfirmResponse,
  TotpDisableRequest,
} from '../../shared/models/auth.dto';

/**
 * Authentication service
 */
@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private readonly storageService = inject(StorageService);
  private readonly apiService = inject(ApiService);
  private readonly router = inject(Router);

  /**
   * Login user
   */
  login(credentials: LoginRequest): Observable<LoginResponse> {
    // Don't use tap for login - handle response validation and storage in subscribe
    // This avoids any potential issues with tap affecting error callback execution context
    return this.apiService.post<LoginResponse>('/auth/login', credentials, { service: 'user' });
  }

  /**
   * 登录第二步:pendingToken + 验证码(或备用码)换正式 token。
   */
  loginTwoFactor(req: LoginTwoFactorRequest): Observable<LoginResponse> {
    return this.apiService.post<LoginResponse>('/auth/login/2fa', req, { service: 'user' });
  }

  /**
   * 开始绑定两步验证(需已登录 + 重验密码)→ 二维码 URI + 密钥。
   */
  setupTotp(req: TotpSetupRequest): Observable<TotpSetupResponse> {
    return this.apiService.post<TotpSetupResponse>('/auth/2fa/setup', req, { service: 'user' });
  }

  /**
   * 确认绑定(输 app 上的码)→ 一次性备用码 + 正式登录态。
   */
  confirmTotp(req: TotpConfirmRequest): Observable<TotpConfirmResponse> {
    return this.apiService.post<TotpConfirmResponse>('/auth/2fa/confirm', req, { service: 'user' });
  }

  /**
   * 关闭两步验证(需密码 + 当前码)。
   */
  disableTotp(req: TotpDisableRequest): Observable<{ success: boolean }> {
    return this.apiService.post<{ success: boolean }>('/auth/2fa/disable', req, { service: 'user' });
  }

  /**
   * 把一次完整登录响应落到本地存储(token + 用户)。
   * 强制绑定场景 confirm 返回的正式 login,以及普通登录/二步登录都走这里,
   * 用来把「仅绑定 token」替换成正式 token、并刷新回显用的 user(含 twoFactorEnabled)。
   */
  applyLoginResponse(response: LoginResponse): void {
    this.storageService.setAccessToken(response.accessToken);
    this.storageService.setRefreshToken(response.refreshToken);
    this.storageService.setUser(response.user || { features: response.features || [] });
  }

  /**
   * 是否处于「未绑定两步验证」门禁态。
   */
  isEnrollmentPending(): boolean {
    return this.storageService.isEnrollmentPending();
  }

  /**
   * Logout user.
   * 先调后端 /auth/logout 把 access(拦截器/header 自动带 Bearer)+ refresh 加入黑名单,使其立即失效
   * ——否则泄露的 token 在自然过期(JWT_EXPIRE_TIME)前仍可被他人使用。无论成败都继续本地登出。
   */
  logout(): void {
    const refreshToken = this.storageService.getRefreshToken() ?? '';
    this.apiService.post('/auth/logout', { refreshToken }, { service: 'user' }).subscribe({
      next: () => this.finishLogout(),
      error: () => this.finishLogout(),
    });
  }

  /** 清本地凭据并跳登录入口(整页刷新,确保内存态一并清干净)。
   *  prd/eng 下 environment.loginUrl='/gate/' → 落到 admin-gate 登录壳(并触发门禁重新校验);dev='/login'。 */
  private finishLogout(): void {
    this.storageService.clear();
    window.location.href = environment.loginUrl;
  }

  /**
   * Check if user is authenticated
   */
  isAuthenticated(): boolean {
    return this.storageService.isAuthenticated();
  }

  /**
   * Get access token
   */
  getAccessToken(): string | null {
    return this.storageService.getAccessToken();
  }

  /**
   * 当前用户的 features。**从 access token 权威解出**(不读 localStorage 的 user 快照 —— 快照会
   * 过期/被清空,导致菜单/路由门控失效)。token 由后端签发时带上角色的 features,刷新后自动更新。
   * 纯用于 UI 门控;真正鉴权在后端每个接口的 JWTAuthMiddleware。
   */
  getUserFeatures(): string[] {
    return extractFeaturesFromToken(this.storageService.getAccessToken());
  }

  /**
   * 是否拥有某 feature(UI 门控用)。'*' 是超管通配:持有即视为拥有一切,含将来新增的页面/接口
   * (见迁移 20260816130000、后端中间件同款判断)。菜单可见性与 featureGuard 都走这里,保持一致。
   */
  hasFeature(feature: string): boolean {
    const features = this.getUserFeatures();
    return features.includes('*') || features.includes(feature);
  }
}

