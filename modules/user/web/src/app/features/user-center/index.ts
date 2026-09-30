/**
 * 用户中心特性模块(user / role / role-feature)对外出口(barrel)。
 *
 * 宿主只依赖这两个导出接入模块(设计纲领 §4「三件套」):
 *   - USER_CENTER_ROUTES  → app.routes.ts 里 `...USER_CENTER_ROUTES` spread
 *   - provideUserCenter() → app.config.ts 的 providers 里加一行
 * 模块内部页面 / DTO / 菜单细节对宿主不可见。
 */
export { USER_CENTER_ROUTES } from './user-center.routes';
export { provideUserCenter } from './provide-user-center';
// Native Federation:暴露给宿主的路由入口(federation.config.js `./Routes` 指向 remote-routes.ts)。
export { REMOTE_USER_CENTER_ROUTES } from './remote-routes';
