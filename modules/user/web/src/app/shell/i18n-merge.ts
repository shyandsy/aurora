/**
 * 特性模块 i18n 合并(ngx-translate 落地实现)。
 *
 * 这是壳层贡献套件里**唯一**与具体 i18n 引擎(ngx-translate)耦合的一小块,单独成文件,
 * 让 `contributions.ts` 的扩展点契约(token / 类型 / provideFeature)保持引擎中性。
 * 换 i18n 引擎时只需替换本文件。
 *
 * 做的事:把一个模块自带的 `Record<locale, TranslationDict>` **深合并**进宿主
 * TranslateService 的各语言字典,采用 `setTranslation(lang, dict, shouldMerge=true)`。
 *
 * 关键:http-loader 是**异步**按语言加载宿主主字典的,且 `use(lang)` 触发的加载会
 * **整体替换**该语言字典(shouldMerge=false),会把先前 merge 进去的模块 key 冲掉。
 * 所以合并时机必须在**每次**语言加载/切换**完成之后**——用 `onLangChange` 订阅,
 * 每当某语言就绪就把模块子树**再追加一次**,任何加载/切换都覆盖不掉模块 key。
 * 同时在初始化时对已提供的各 locale 立即 merge 一遍,兜住订阅注册前可能已发生的首个
 * 语言事件(初始化里同步订阅先于 http 异步回调,故正常也能被 onLangChange 捕获,
 * 立即 merge 只是双保险)。
 *
 * 用 **ENVIRONMENT_INITIALIZER 而非 APP_INITIALIZER**(关键,Native Federation 场景):
 *   - 独立运行(web/user 自己启动):本 provider 在 app.config 根 providers 里 → 根环境注入器创建时
 *     即运行,时机与 APP_INITIALIZER 等价(且订阅注册更早,更稳)。
 *   - 被 admin 宿主 federated:宿主只把本模块暴露的 `./Routes` 挂进自己的路由,provideUserCenter 是
 *     挂在那条**路由级 providers** 上的。APP_INITIALIZER 只在 app 引导期跑一次、**不会**为路由级注入器
 *     执行,故那时 i18n 永远合并不进来;而 ENVIRONMENT_INITIALIZER 在**任何**环境注入器(含路由懒加载
 *     子树)创建时都会执行 —— 用户点进 /user-center 加载 remote 路由的一刻,就把模块词典合并进**宿主的**
 *     TranslateService(ngx-translate 是 federation 共享单例,inject 解析到宿主根的同一个实例)。
 */
import { ENVIRONMENT_INITIALIZER, inject, Provider } from '@angular/core';
import { TranslateService } from '@ngx-translate/core';
import { TranslationDict } from './contributions';

/**
 * 生成把 `i18n` 深合并进(当前注入器可见的)ngx-translate 的 provider(ENVIRONMENT_INITIALIZER multi)。
 * 可被多个模块各自调用,彼此独立、幂等。
 */
export function provideFeatureI18nMerge(
  i18n: Record<string, TranslationDict>,
): Provider {
  return {
    provide: ENVIRONMENT_INITIALIZER,
    multi: true,
    useValue: () => {
      // 解析到当前注入上下文可见的 TranslateService:独立=根;federated=沿路由注入器链上溯到宿主根的
      // 共享单例(与宿主 UI 用的同一个实例),故合并即刻反映到宿主界面。
      const translate = inject(TranslateService);
      const applyLang = (lang: string): void => {
        const dict = i18n[lang];
        if (dict) {
          // shouldMerge=true → 深合并,不清掉该语言已有(宿主/其它模块)的 key。
          translate.setTranslation(lang, dict, true);
        }
      };
      // 1) 立即对模块提供的每种语言 merge 一遍(兜住当前/已加载语言 —— federated 时宿主通常已加载好当前语言)。
      Object.keys(i18n).forEach(applyLang);
      // 2) 之后每次某语言被(重新)加载/切换完成时再追加,防止被主字典 http-loader 替换覆盖。
      translate.onLangChange.subscribe((e) => applyLang(e.lang));
    },
  };
}
