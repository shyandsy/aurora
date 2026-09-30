import { Routes } from '@angular/router';
import { twoFactorEnrollGuard } from './core/guards/two-factor-enroll.guard';
import { USER_CENTER_ROUTES } from './features/user-center';

/**
 * web/user 路由表(用户中心独立后台)。
 *
 * 只挂:登录、强制两步验证绑定门禁页、用户中心特性模块(账号 / 角色权限 / Microservice 三个 tab,
 * 见 features/user-center)。不含任何业务模块,也**没有独立的首页落地页**——用户中心是有界工具,
 * 独立访问直接落到用户中心(账号 tab),不需要卡片式落地页。
 */
export const routes: Routes = [
  {
    path: 'login',
    loadComponent: () => import('./pages/login/login.component').then(m => m.LoginComponent)
  },
  {
    // 强制两步验证但账号未绑定时的门禁页(自身守卫:仅门禁态可进,别的情况弹走)。
    path: 'two-factor-enroll',
    loadComponent: () => import('./pages/two-factor-enroll/two-factor-enroll.component').then(m => m.TwoFactorEnrollComponent),
    canActivate: [twoFactorEnrollGuard]
  },
  // 用户中心(user-permission:账号 / 角色权限 / Microservice 三个 tab)路由由特性模块贡献,
  // 见 features/user-center。microservice 现为壳的第 3 个 tab(user-permission/microservice)。
  ...USER_CENTER_ROUTES,
  {
    // 兼容出口:microservice 已收成用户中心第 3 个 tab;旧独立顶层路由 / 旧书签重定向到壳内 tab。
    path: 'microservice',
    redirectTo: 'user-permission/microservice',
    pathMatch: 'full'
  },
  // 根路径 / 及旧 /home:直接进用户中心(无落地页)。
  { path: '', redirectTo: 'user-permission', pathMatch: 'full' },
  { path: 'home', redirectTo: 'user-permission', pathMatch: 'full' },
  {
    path: '**',
    redirectTo: 'user-permission'
  }
];
