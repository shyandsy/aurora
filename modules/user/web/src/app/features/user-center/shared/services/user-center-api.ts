import { Injectable, inject, InjectionToken, NgZone } from '@angular/core';
import { HttpClient, HttpHeaders, HttpErrorResponse } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError, timeout } from 'rxjs/operators';
import { StorageService } from '../../../../core/services/storage.service';
import { ErrorService } from '../../../../core/services/error.service';
import { extractErrorMessage } from '../../../../shared/utils/error.util';
import { environment } from '../../../../../environments/environment';
import { PagingResponse } from '@common/models/common.dto';
import {
  MicroserviceTokenFeature,
  CreateMicroserviceTokenFeatureRequest,
  UpdateMicroserviceTokenFeatureRequest,
  IssueMicroserviceTokenRequest,
  IssueMicroserviceTokenResponse,
  GetMicroserviceTokenTokensRequest,
  MicroserviceTokenFeatureToken,
} from '../models/microservice.dto';

/**
 * 用户中心模块自带的 HTTP 访问层的 baseUrl 注入点。
 *
 * 宿主 `provideUserCenter({ baseUrl })` 注入(= environment.userApiBaseUrl,与旧
 * god ApiService 对 `service:'user'` 解析出的同一个 base)。模块内的 UserCenterApi 据此拼 URL,
 * 从而**不再依赖宿主 god ApiService** 的 resolveBase/ApiTarget。
 */
export const USER_CENTER_BASE_URL = new InjectionToken<string>('USER_CENTER_BASE_URL');

/** UserCenterApi 请求可选项(与旧 god ApiService 的同名字段语义一致)。 */
export interface UserCenterRequestOptions {
  /** 自定义超时(毫秒),覆盖全局 environment.apiTimeout。 */
  timeoutMs?: number;
}

/**
 * 用户中心模块自带的瘦 HTTP 访问层(单一出口)。
 *
 * 逐字复刻旧 god `ApiService` 的通用 get/post/put/delete 语义:
 *   - getHeaders:Content-Type + Bearer(与旧实现一致;全局 authInterceptor 另在 401 重放时覆盖头);
 *   - timeout(environment.apiTimeout)/ post 支持 options.timeoutMs;
 *   - handleError:401/403 透传给 authInterceptor,其余走全局错误框 + 重抛;
 *   - post 对 /auth/login、/auth/2fa 内联表单错误不走全局错误框(与旧实现一致)。
 * 唯一区别:baseUrl 固定为注入的 USER_CENTER_BASE_URL(旧实现按 `service:'user'` 解析出同一个),
 * 因此调用方不再传 `{ service: 'user' }`。走的仍是 Angular HttpClient,全局 auth interceptor 照常生效。
 */
// 注意:**不用** providedIn:'root'。UserCenterApi 依赖路由级 provide 的 USER_CENTER_BASE_URL
// (由 provideUserCenter 提供)。若放 root,federated 进宿主时它在 root 实例化、看不到路由级
// 的 USER_CENTER_BASE_URL → NG0201。故改由 provideUserCenter 与 baseUrl 同层 provide,
// 独立(app.config)与 federated(路由级)两种挂载都在同一注入器内解析到。
@Injectable()
export class UserCenterApi {
  private readonly http = inject(HttpClient);
  private readonly storageService = inject(StorageService);
  private readonly errorService = inject(ErrorService);
  private readonly ngZone = inject(NgZone);
  private readonly baseUrl = inject(USER_CENTER_BASE_URL);

  /** 默认请求头(带 Authorization),与旧 god ApiService.getHeaders 一致。 */
  private getHeaders(): HttpHeaders {
    let headers = new HttpHeaders({
      'Content-Type': 'application/json',
    });

    const token = this.storageService.getAccessToken();
    if (token) {
      headers = headers.set('Authorization', `Bearer ${token}`);
    }

    return headers;
  }

  /** 拼完整 URL:绝对地址原样使用;否则前缀模块 baseUrl。 */
  private buildUrl(endpoint: string): string {
    if (endpoint.startsWith('http://') || endpoint.startsWith('https://')) {
      return endpoint;
    }
    return `${this.baseUrl}${endpoint}`;
  }

  get<T>(endpoint: string): Observable<T> {
    return this.http
      .get<T>(this.buildUrl(endpoint), { headers: this.getHeaders() })
      .pipe(timeout(environment.apiTimeout), catchError(this.handleError.bind(this)));
  }

  post<T>(endpoint: string, body: any, options?: UserCenterRequestOptions): Observable<T> {
    // 登录及两步验证(login/2fa)的错误一律回组件内联展示,不弹全局错误框(与旧实现一致)。
    const isAuthInlineRequest = endpoint.includes('/auth/login') || endpoint.includes('/auth/2fa');
    const obs = this.http.post<T>(this.buildUrl(endpoint), body, { headers: this.getHeaders() });

    if (isAuthInlineRequest) {
      return obs;
    }

    return obs.pipe(
      timeout(options?.timeoutMs ?? environment.apiTimeout),
      catchError(this.handleError.bind(this)),
    );
  }

  put<T>(endpoint: string, body: any): Observable<T> {
    return this.http
      .put<T>(this.buildUrl(endpoint), body, { headers: this.getHeaders() })
      .pipe(timeout(environment.apiTimeout), catchError(this.handleError.bind(this)));
  }

  delete<T>(endpoint: string): Observable<T> {
    return this.http
      .delete<T>(this.buildUrl(endpoint), { headers: this.getHeaders() })
      .pipe(timeout(environment.apiTimeout), catchError(this.handleError.bind(this)));
  }

  /** 错误处理:401/403 交给 authInterceptor,其余弹全局错误框并重抛(与旧 god ApiService 一致)。 */
  private handleError(error: HttpErrorResponse): Observable<never> {
    if (error.status === 401 || error.status === 403) {
      return throwError(() => error);
    }

    const message = extractErrorMessage(error);
    this.ngZone.run(() => {
      this.errorService.showError(message);
    });

    return throwError(() => error);
  }

  // ---- Microservice Token Feature APIs -------------------------------------
  // 从旧 god ApiService **原样搬入**(端点 URL / method / body / 返回类型逐字不变);
  // 旧实现走 { service: 'user' } → userApiBaseUrl,这里 baseUrl 已固定为同一个,故 URL 一致。
  getMicroserviceTokenFeatures(): Observable<MicroserviceTokenFeature[]> {
    return this.get<MicroserviceTokenFeature[]>(`/microservice`);
  }

  getMicroserviceTokenFeature(id: number): Observable<MicroserviceTokenFeature> {
    return this.get<MicroserviceTokenFeature>(`/microservice/${id}`);
  }

  createMicroserviceTokenFeature(req: CreateMicroserviceTokenFeatureRequest): Observable<MicroserviceTokenFeature> {
    return this.post<MicroserviceTokenFeature>(`/microservice`, req);
  }

  updateMicroserviceTokenFeature(id: number, req: UpdateMicroserviceTokenFeatureRequest): Observable<MicroserviceTokenFeature> {
    return this.put<MicroserviceTokenFeature>(`/microservice/${id}`, req);
  }

  deleteMicroserviceTokenFeature(id: number): Observable<{ message: string }> {
    return this.delete<{ message: string }>(`/microservice/${id}`);
  }

  // ---- Microservice Token APIs ---------------------------------------------
  issueMicroserviceToken(req: IssueMicroserviceTokenRequest): Observable<IssueMicroserviceTokenResponse> {
    return this.post<IssueMicroserviceTokenResponse>(`/microservice/jwt_token`, req);
  }

  getMicroserviceTokenTokens(req: GetMicroserviceTokenTokensRequest): Observable<PagingResponse<MicroserviceTokenFeatureToken>> {
    const params = new URLSearchParams();
    params.append('page', req.page.toString());
    params.append('pageSize', req.pageSize.toString());
    return this.get<PagingResponse<MicroserviceTokenFeatureToken>>(`/microservice/jwt_token?${params.toString()}`);
  }

  getMicroserviceTokenToken(id: number): Observable<MicroserviceTokenFeatureToken> {
    return this.get<MicroserviceTokenFeatureToken>(`/microservice/jwt_token/${id}`);
  }

  enableMicroserviceToken(id: number): Observable<{ message: string }> {
    return this.put<{ message: string }>(`/microservice/jwt_token/${id}/enable`, {});
  }

  disableMicroserviceToken(id: number): Observable<{ message: string }> {
    return this.put<{ message: string }>(`/microservice/jwt_token/${id}/disable`, {});
  }
}
