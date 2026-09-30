/**
 * 用户中心模块自带的三语翻译包(模块 i18n 自包含,设计纲领 §5)。
 *
 * 这些 key 的**值从宿主全局字典里原样搬出**(user / role / roleFeature / rolePermission /
 * microservice 各页文案 + 侧栏 `sidebar.userPermission` 及页内 tab `sidebar.users` /
 * `sidebar.roles`)。key 名**不改**(低风险版:仍是原命名,不加命名空间前缀),渲染逐字不变。
 *
 * 由 `provideUserCenter` → `provideFeature({ i18n })` 在启动期**深合并**进宿主 ngx-translate
 * 字典(见 shell/i18n-merge.ts)。合并对每种语言在其 http-loader 加载完成后再追加,
 * 保证不被主字典加载覆盖。
 *
 * 注意:各语言子树按宿主当时的实际内容**逐字复制**,包括宿主里本就缺失的条目
 *(例如 en 缺 `user.create` 等)——刻意保留缺口以保证渲染与搬出前**完全一致**,
 * 不新增未翻译串。补齐是后续 i18n 精修的事,本步不做。
 */
import { TranslationDict } from '../../../shell/contributions';
import zhCN from './zh-CN.json';
import zhTW from './zh-TW.json';
import en from './en.json';

/** locale → 模块翻译子树。locale 名与宿主 SupportedLanguage 一致。 */
export const USER_CENTER_I18N: Record<string, TranslationDict> = {
  'zh-CN': zhCN as TranslationDict,
  'zh-TW': zhTW as TranslationDict,
  'en': en as TranslationDict,
};
