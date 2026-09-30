import { Injectable, inject, NgZone } from '@angular/core';
import { HttpClient, HttpHeaders, HttpErrorResponse } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError, timeout } from 'rxjs/operators';
import { StorageService } from '../../core/services/storage.service';
import { ErrorService } from '../../core/services/error.service';
import { extractErrorMessage } from '../utils/error.util';
import { environment } from '../../../environments/environment';

/**
 * Target backend service for a request.
 * - 'user' -> environment.userApiBaseUrl(用户中心:认证/RBAC/microservice token)——本 app 的主后端。
 *
 * 保留 service 选项(与其它前端 app 的 ApiService 同签名),便于本 app 的调用点(如 AuthService)
 * 逐字复用;本 app 目前只对用户中心后端发请求。
 */
export type ApiTarget = 'user';

/**
 * Common request options for ApiService methods.
 */
export interface ApiRequestOptions {
  /** Which backend service to target. Defaults to 'user'. */
  service?: ApiTarget;
  /** 自定义超时(毫秒),覆盖全局 environment.apiTimeout。 */
  timeoutMs?: number;
}

/**
 * API service for making HTTP requests
 * Handles all non-401 errors (400, 500+, network, etc.)
 * 401/403 errors are handled by authInterceptor
 *
 * Base URL concatenation happens ONLY here. Callers must pass a relative
 * endpoint (e.g. '/auth/login') and select the target service via options,
 * never prepend environment.*BaseUrl themselves.
 */
@Injectable({
  providedIn: 'root'
})
export class ApiService {
  private readonly http = inject(HttpClient);
  private readonly storageService = inject(StorageService);
  private readonly errorService = inject(ErrorService);
  private readonly ngZone = inject(NgZone);

  /**
   * Get default headers with authorization
   */
  private getHeaders(): HttpHeaders {
    let headers = new HttpHeaders({
      'Content-Type': 'application/json'
    });

    const token = this.storageService.getAccessToken();
    if (token) {
      headers = headers.set('Authorization', `Bearer ${token}`);
    }

    return headers;
  }

  /**
   * Resolve the base URL for the given target service.
   * This is the single place where a base URL is chosen.
   */
  private resolveBase(_target: ApiTarget = 'user'): string {
    return environment.userApiBaseUrl;
  }

  /**
   * Build the full request URL from a relative endpoint and the target base.
   * An absolute endpoint (http/https) is used as-is (pure escape hatch); the
   * base selection never depends on this.
   */
  private buildUrl(endpoint: string, options?: ApiRequestOptions): string {
    if (endpoint.startsWith('http://') || endpoint.startsWith('https://')) {
      return endpoint;
    }
    return `${this.resolveBase(options?.service)}${endpoint}`;
  }

  /**
   * GET request
   */
  get<T>(endpoint: string, options?: ApiRequestOptions): Observable<T> {
    const url = this.buildUrl(endpoint, options);
    return this.http.get<T>(url, {
      headers: this.getHeaders()
    }).pipe(
      timeout(environment.apiTimeout),
      catchError(this.handleError.bind(this))
    );
  }

  /**
   * POST request
   */
  post<T>(endpoint: string, body: any, options?: ApiRequestOptions): Observable<T> {
    // 登录及两步验证(setup/confirm/disable/login-2fa)的错误(密码/验证码错等)
    // 一律回到组件内联展示,不弹全局错误框——这些是敏感表单,错误要就地提示。
    const isAuthInlineRequest = endpoint.includes('/auth/login') || endpoint.includes('/auth/2fa');
    const url = this.buildUrl(endpoint, options);
    const obs = this.http.post<T>(url, body, {
      headers: this.getHeaders()
    });

    // For login requests, don't catch errors - let them pass through to component
    if (isAuthInlineRequest) {
      return obs;
    }

    return obs.pipe(
      timeout(options?.timeoutMs ?? environment.apiTimeout),
      catchError(this.handleError.bind(this))
    );
  }

  /**
   * PUT request
   */
  put<T>(endpoint: string, body: any, options?: ApiRequestOptions): Observable<T> {
    const url = this.buildUrl(endpoint, options);
    return this.http.put<T>(url, body, {
      headers: this.getHeaders()
    }).pipe(
      timeout(environment.apiTimeout),
      catchError(this.handleError.bind(this))
    );
  }

  /**
   * DELETE request
   */
  delete<T>(endpoint: string, options?: ApiRequestOptions): Observable<T> {
    const url = this.buildUrl(endpoint, options);
    return this.http.delete<T>(url, {
      headers: this.getHeaders()
    }).pipe(
      timeout(environment.apiTimeout),
      catchError(this.handleError.bind(this))
    );
  }

  /**
   * Handle HTTP errors (non-401, non-403, non-login)
   * 401 and 403 errors are handled by authInterceptor
   * Login requests are handled directly by components (no catchError)
   */
  private handleError(error: HttpErrorResponse): Observable<never> {
    // Skip 401 and 403 errors - they are handled by authInterceptor
    if (error.status === 401 || error.status === 403) {
      return throwError(() => error);
    }

    // Handle all other errors (400, 500+, network, etc.)
    // API returns errors in unified format: { "message": "..." }
    const message = extractErrorMessage(error);
    // Use NgZone.run() to ensure error display happens within Angular's zone
    this.ngZone.run(() => {
      this.errorService.showError(message);
    });

    return throwError(() => error);
  }
}
