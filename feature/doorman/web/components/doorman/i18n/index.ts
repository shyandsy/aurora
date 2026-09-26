/**
 * doorman 模块自带的三语翻译包(模块 i18n 自包含)。
 *
 * 由 DoormanConfigComponent 在初始化时**深合并**进(独立=根 / federated=宿主共享单例)ngx-translate
 * 的各语言字典,并订阅 onLangChange 在每次语言加载/切换后再追加一次,防止被宿主主字典的 http-loader
 * 整体替换覆盖(doorman 暴露的是组件、不是路由,故在组件里就地深合并,而非用路由级 initializer)。
 */
import zhCN from './zh-CN.json';
import zhTW from './zh-TW.json';
import en from './en.json';

/** 任意深度的翻译树。 */
export type TranslationDict = { [key: string]: string | TranslationDict };

/** locale → 模块翻译子树。locale 名与宿主 SupportedLanguage 一致('zh-CN' / 'zh-TW' / 'en')。 */
export const DOORMAN_I18N: Record<string, TranslationDict> = {
  'zh-CN': zhCN as TranslationDict,
  'zh-TW': zhTW as TranslationDict,
  'en': en as TranslationDict,
};
