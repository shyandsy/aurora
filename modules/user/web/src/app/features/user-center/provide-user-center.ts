import { EnvironmentProviders, makeEnvironmentProviders } from '@angular/core';
import { provideFeature } from '../../shell/contributions';
import { USER_CENTER_BASE_URL, UserCenterApi } from './shared/services/user-center-api';
import { USER_CENTER_I18N } from './i18n';

/** provideUserCenter 的接入配置。 */
export interface UserCenterConfig {
  /**
   * 用户中心后端(认证/RBAC/microservice token)的 API base URL。
   * 宿主传 `environment.userApiBaseUrl` —— 与旧 god ApiService 对 `service:'user'` 解析出的同一个。
   * 模块自带的 UserCenterApi 据此拼 URL,不再依赖宿主 god ApiService。
   */
  baseUrl: string;
}

/**
 * 把用户中心模块的全部「接线」(自带 HTTP 层 baseUrl + i18n 词典合并)一次性注册进宿主。
 * 独立运行:`app.config.ts` 的 providers 里加一行;federated:挂在 remote-routes 的路由级 providers 上。
 * 用户中心是有界工具、无 sidebar,故不再贡献菜单 / 页面标签(那套只有 sidebar 会消费,已删)。
 */
export function provideUserCenter(config: UserCenterConfig): EnvironmentProviders {
  return makeEnvironmentProviders([
    { provide: USER_CENTER_BASE_URL, useValue: config.baseUrl },
    // 与 baseUrl 同层 provide,不用 providedIn:'root'(见 user-center-api.ts 注释):
    // federated 时它随本路由子树的注入器创建,能解析到路由级的 USER_CENTER_BASE_URL。
    UserCenterApi,
    // 模块自带三语翻译包,启动期深合并进(独立=根 / federated=宿主共享单例)ngx-translate。
    // federated 下用户中心文案就靠这条链路,别删(见 i18n/index.ts + shell/i18n-merge.ts)。
    provideFeature({ i18n: USER_CENTER_I18N }),
  ]);
}
