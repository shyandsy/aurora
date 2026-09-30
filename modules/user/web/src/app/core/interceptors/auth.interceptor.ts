import {
  HttpInterceptorFn,
  HttpErrorResponse,
  HttpRequest,
  HttpHandlerFn,
  HttpEvent,
  HttpClient,
} from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { Observable, BehaviorSubject, catchError, filter, switchMap, take, throwError } from 'rxjs';
import { StorageService } from '../services/storage.service';
import { ErrorService } from '../services/error.service';
import { environment } from '../../../environments/environment';

/**
 * 认证拦截器:
 * - 401(access 过期):用 refreshToken 静默换新 access+refresh(后端 /auth/refresh 轮换)再重放原请求;
 *   没有 refreshToken 或续期失败(refresh 也过期/被拉黑)才清本地并跳登录。并发只发一次 refresh。
 * - 403(已登录但无权限):弹权限错误(去重),不动登录态。
 *
 * /auth/* 一律放行,由各自流程处理,也避免 refresh 递归。
 */

// 模块级共享状态(SPA 单例)。refreshResult:null=刷新进行中;string=新 access token;false=刷新失败。
let isRefreshing = false;
const refreshResult = new BehaviorSubject<string | false | null>(null);

interface RefreshResp {
  accessToken: string;
  refreshToken: string;
}

export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const router = inject(Router);
  const storage = inject(StorageService);
  const errorService = inject(ErrorService);
  const http = inject(HttpClient);

  if (req.url.includes('/auth/')) {
    return next(req);
  }

  return next(req).pipe(
    catchError((error: HttpErrorResponse) => {
      if (error.status === 401) {
        return handle401(req, next, http, storage, router);
      }
      if (error.status === 403) {
        const errorMessage = error.error?.message || 'You do not have permission to access this resource';
        errorService.showForbiddenError(errorMessage);
        return throwError(() => error);
      }
      return throwError(() => error);
    }),
  );
};

/** 用新 token 重放请求(Authorization 头由本拦截器直接覆盖)。 */
function withToken(req: HttpRequest<unknown>, token: string): HttpRequest<unknown> {
  return req.clone({ setHeaders: { Authorization: `Bearer ${token}` } });
}

/** 续期彻底失败 → 清本地凭据 + 跳登录入口。
 *  用整页导航到 environment.loginUrl(prd/eng='/gate/' 登录壳)而非 router.navigate:
 *  客户端路由不会离开已加载的 SPA、也不触发 forwardAuth;整页跳转才会让门禁重新校验、把人挡回壳子。 */
function forceLogin(storage: StorageService, router: Router, error: unknown): Observable<never> {
  // 先记下用户当前所在页,门禁整页往返后 App 启动会据此跳回,而不是一律落到首页。
  // clear() 不动 RETURN_URL_KEY,故先后顺序都安全;放在 clear 后更直白。
  storage.clear();
  storage.saveReturnUrl(router.url);
  window.location.href = environment.loginUrl;
  return throwError(() => error);
}

function handle401(
  req: HttpRequest<unknown>,
  next: HttpHandlerFn,
  http: HttpClient,
  storage: StorageService,
  router: Router,
): Observable<HttpEvent<unknown>> {
  const refreshToken = storage.getRefreshToken();
  if (!refreshToken) {
    return forceLogin(storage, router, new Error('no refresh token'));
  }

  if (isRefreshing) {
    return refreshResult.pipe(
      filter((v): v is string | false => v !== null),
      take(1),
      switchMap((v) => (v === false ? forceLogin(storage, router, new Error('refresh failed')) : next(withToken(req, v)))),
    );
  }

  isRefreshing = true;
  refreshResult.next(null);

  return http.post<RefreshResp>(`${environment.userApiBaseUrl}/auth/refresh`, { refreshToken }).pipe(
    switchMap((resp) => {
      isRefreshing = false;
      storage.setAccessToken(resp.accessToken);
      if (resp.refreshToken) {
        storage.setRefreshToken(resp.refreshToken);
      }
      refreshResult.next(resp.accessToken);
      return next(withToken(req, resp.accessToken));
    }),
    catchError((err) => {
      isRefreshing = false;
      refreshResult.next(false); // 通知排队请求:刷新失败,别再干等
      return forceLogin(storage, router, err);
    }),
  );
}
