import { Routes } from '@angular/router';
import { provideUserCenter } from './provide-user-center';
import { USER_CENTER_ROUTES } from './user-center.routes';
import { environment } from '../../../environments/environment';

/**
 * Native Federation 暴露给宿主(host 后台壳)的用户中心路由入口(`exposes: { './Routes': ... }`)。
 *
 * 宿主用 `loadRemoteModule('user','./Routes').then(m => m.REMOTE_USER_CENTER_ROUTES)` 作为一条
 * 懒加载路由的 loadChildren,把整棵用户中心子树挂进宿主内容区的 router-outlet。
 *
 * 与「独立运行」(web/user 自己 AppComponent + sidebar 包 USER_CENTER_ROUTES)的关键区别 ——
 * 宿主**只加载这份暴露的路由**,不加载 web/user 的 AppComponent,所以:
 *   1) web/user 自己的 sidebar/topbar/壳**不会**进宿主(无双侧栏);宿主只见到这里挂的组件树,
 *      即 user-permission(顶部 tab)及其子页,不含任何 web/user 布局。
 *   2) 独立运行所需的「模块接线」(自带 HTTP 层 baseUrl + i18n 深合并)在独立时由 app.config 的
 *      provideUserCenter 提供;federated 时宿主 app.config 里没有它,故必须把它挂在**本路由的
 *      route-level providers** 上,随这棵子树被加载时一并生效。i18n 合并因此改用 ENVIRONMENT_INITIALIZER
 *      (见 shell/i18n-merge.ts),路由级注入器创建即执行。
 *
 * baseUrl 取本 remote 自己的 environment.userApiBaseUrl(prd 为相对 `/api/user/v1`,与 admin 同域,
 * 故 federated 进宿主后请求同域用户后端,正确)。
 *
 * 鉴权:USER_CENTER_ROUTES 上的 authGuard/featureGuard 用 remote 自己 bundle 的 guard + AuthService/
 * StorageService(providedIn:'root'),读同域 localStorage 的 admin_access_token(与宿主同一份 token)。
 */
export const REMOTE_USER_CENTER_ROUTES: Routes = [
  {
    path: '',
    providers: [
      provideUserCenter({ baseUrl: environment.userApiBaseUrl }),
    ],
    children: [
      ...USER_CENTER_ROUTES,
      // 宿主把本子树挂在 /user-center 下;裸访问 /user-center 时兜进用户与权限页。
      { path: '', redirectTo: 'user-permission', pathMatch: 'full' },
    ],
  },
];
