import { ApplicationConfig, provideBrowserGlobalErrorListeners, importProvidersFrom, APP_INITIALIZER } from '@angular/core';
import { provideRouter } from '@angular/router';
import { provideHttpClient, withInterceptors, HttpClient } from '@angular/common/http';
import { TranslateService } from '@ngx-translate/core';
import { TranslateModule } from '@ngx-translate/core';
import { provideTranslateHttpLoader } from '@ngx-translate/http-loader';

import { routes } from './app.routes';
import { authInterceptor } from './core/interceptors/auth.interceptor';
import { languageInterceptor } from './core/interceptors/language.interceptor';
import { provideUserCenter } from './features/user-center';
import { environment } from '../environments/environment';

export function appInitializerFactory(translateService: TranslateService, httpClient: HttpClient) {
  return () => {
    const defaultLang = 'zh-CN';
    return new Promise<void>((resolve) => {
      translateService.addLangs([defaultLang]);
      // Manually load translation file to avoid issue where use() doesn't properly load for default language
      httpClient.get(`/assets/i18n/${defaultLang}.json`).subscribe({
        next: (translations: any) => {
          translateService.setTranslation(defaultLang, translations, true);
          translateService.use(defaultLang).subscribe({
            next: () => resolve(),
            error: () => resolve()
          });
        },
        error: () => {
          translateService.use(defaultLang).subscribe({
            next: () => resolve(),
            error: () => resolve()
          });
        }
      });
    });
  };
}

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    provideRouter(routes),
    provideHttpClient(withInterceptors([languageInterceptor, authInterceptor])),
    importProvidersFrom(
      TranslateModule.forRoot({
        fallbackLang: 'zh-CN'
      })
    ),
    provideTranslateHttpLoader({
      prefix: '/assets/i18n/',
      suffix: '.json'
    }),
    {
      provide: APP_INITIALIZER,
      useFactory: appInitializerFactory,
      deps: [TranslateService, HttpClient],
      multi: true
    },
    // 用户中心特性模块接入(自带 HTTP 层 baseUrl + i18n 词典深合并)。
    // baseUrl 传 environment.userApiBaseUrl —— 与旧 god ApiService 对 `service:'user'` 用的同一个。
    provideUserCenter({ baseUrl: environment.userApiBaseUrl })
  ]
};
