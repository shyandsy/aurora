import { Routes } from '@angular/router';
import { authGuard } from '../../core/guards/auth.guard';
import { featureGuard } from '../../core/guards/feature.guard';

/**
 * 用户中心特性模块的路由子树(账号 user / 角色 role / 权限 role-feature)。
 *
 * 内容从宿主 `app.routes.ts` **原样搬出**:path / loadComponent / data.feature / guard 逐字保留,
 * 仅 loadComponent 路径与 guard import 因文件搬进本模块而相对化到本目录(指向的组件、守卫完全相同)。
 * 宿主用 `...USER_CENTER_ROUTES` 一行 spread 接入(设计纲领 §4「三件套」之一、§9 路由靠静态 spread)。
 *
 * 注:两条旧扁平路由重定向(/user、/role-feature)也随本子树搬出——它们是本模块的兼容出口,
 * spread 进宿主根路由后仍是顶层 path,行为不变。
 */
export const USER_CENTER_ROUTES: Routes = [
  {
    // 用户与权限:壳组件带 tab,子页各占一个 tab。父路由只守 authGuard(+featureGuard)。
    path: 'user-permission',
    loadComponent: () => import('./pages/user-permission/user-permission.component').then(m => m.UserPermissionComponent),
    canActivate: [authGuard, featureGuard],
    data: { feature: 'ui.menu.user_permission' },
    children: [
      {
        path: 'users',
        loadComponent: () => import('./pages/user/user.component').then(m => m.UserComponent)
      },
      {
        path: 'roles',
        loadComponent: () => import('./pages/role-feature/role-feature.component').then(m => m.RoleFeatureComponent)
      },
      {
        // 第 3 个 tab:Microservice(功能配置 + 签发 service token)。父 authGuard 已保证登录;
        // 这里再挂 featureGuard(ui.menu.microservice),直达 URL 也守得住(与壳 tab 门控同一 feature)。
        path: 'microservice',
        loadComponent: () => import('./pages/microservice/microservice.component').then(m => m.MicroserviceComponent),
        canActivate: [featureGuard],
        data: { feature: 'ui.menu.microservice' }
      },
      {
        // 第 4 个 tab:被锁登录 / 登录限流(loginguard 后台管理)。父 authGuard 已保证登录;
        // 门控用后端**实际校验的** feature `user.get`(查看语义,列表接口正是这个)——后端刻意
        // 复用用户权限、未新增 ui.menu.*,故不另造门控 key(否则要等后端 seed 才可见)。
        // 与壳 tab 门控同一 feature,直达 URL 也守得住。
        path: 'locked-logins',
        loadComponent: () => import('./pages/locked-logins/locked-logins.component').then(m => m.LockedLoginsComponent),
        canActivate: [featureGuard],
        data: { feature: 'user.get' }
      },
      { path: '', redirectTo: 'users', pathMatch: 'full' }
    ]
  },
  // ---- 旧路由重定向(兼容书签和代码里的旧跳转)---------------------------------
  { path: 'user', redirectTo: 'user-permission/users', pathMatch: 'full' },
  { path: 'role-feature', redirectTo: 'user-permission/roles', pathMatch: 'full' }
];
