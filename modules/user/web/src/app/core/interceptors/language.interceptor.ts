import { HttpInterceptorFn } from '@angular/common/http';

/**
 * HTTP interceptor to set Accept-Language header based on user's selected language
 */
export const languageInterceptor: HttpInterceptorFn = (req, next) => {

  let currentLang = localStorage.getItem('admin_language');
  if (!currentLang || !(currentLang === 'en' || currentLang === 'zh-CN' || currentLang === 'zh-TW')) {
    currentLang = 'zh-CN';
  }

  let acceptLanguage: string;
  switch (currentLang) {
    case 'en':
      acceptLanguage = 'en-US,en;q=0.9';
      break;
    case 'zh-CN':
      acceptLanguage = 'zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7';
      break;
    case 'zh-TW':
      acceptLanguage = 'zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7';
      break;
    default:
      acceptLanguage = 'zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7';
  }

  const clonedReq = req.clone({
    setHeaders: {
      'Accept-Language': acceptLanguage
    }
  });

  return next(clonedReq);
};

