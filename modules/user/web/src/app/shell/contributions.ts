/**
 * Shell Contribution Kit(壳层贡献套件)
 *
 * 宿主壳只在这里定义一次「扩展点」;任何特性模块用统一的贡献方式把自己的接线贡献进来。
 * 本文件**中性、零业务依赖**,只依赖 @angular/core。
 *
 * web/user 是有界的「用户中心」工具,只有顶栏 + 页面内顶部 tab 导航、**无左侧 sidebar**,
 * 故套件里只保留 federated / 独立两种场景都需要的 **i18n 词典合并**这一贡献维度;
 * 原先给 sidebar 用的菜单(MENU_CONTRIBUTIONS)、RBAC 页面标签(PAGE_LABEL_CONTRIBUTIONS)、
 * 门禁契约(FEATURE_ACCESS)贡献点在 sidebar 删除后已无任何消费者,一并移除。
 *
 * 设计纲领:宿主后台壳的可插拔模块化(贡献套件)。
 */
import {
  EnvironmentProviders,
  makeEnvironmentProviders,
} from '@angular/core';
// i18n 合并是套件里唯一与具体 i18n 引擎(ngx-translate)耦合的一小块,隔离在单独文件里;
// 本文件其余部分保持引擎中性。
import { provideFeatureI18nMerge } from './i18n-merge';

/**
 * 一份 i18n 命名空间字典:{ 'zh-CN': { ... }, 'en': { ... } }。
 * 值是任意深度的翻译树,故用宽松类型;kit 不依赖具体 i18n 引擎。
 */
export type TranslationDict = { [key: string]: string | TranslationDict };

/** 一个特性模块交付给宿主的全部「接线」。 */
export interface FeatureContribution {
  /** 模块自带的命名空间字典;合并进宿主 i18n(见 ./i18n-merge)。 */
  i18n?: Record<string, TranslationDict>;
}

/**
 * 把一个模块的 i18n 一次性注册进宿主。
 * 一个模块的全部接线收口到这一个调用里(设计纲领 §3.2)。
 */
export function provideFeature(c: FeatureContribution): EnvironmentProviders {
  const providers = [];

  // i18n:把模块自带的 { locale: TranslationDict } 深合并进宿主 ngx-translate 字典。
  // 合并时机 + 防被 http-loader 覆盖的策略见 ./i18n-merge。
  if (c.i18n) {
    providers.push(provideFeatureI18nMerge(c.i18n));
  }

  return makeEnvironmentProviders(providers);
}
