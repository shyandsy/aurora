import { inject } from '@angular/core';
import { Router, CanActivateFn } from '@angular/router';
import { StorageService } from '../services/storage.service';

/**
 * Auth guard to protect routes
 * Redirects to login if user is not authenticated
 */
export const authGuard: CanActivateFn = (route, state) => {
  const storageService = inject(StorageService);
  const router = inject(Router);

  if (!storageService.isAuthenticated()) {
    // 存下用户本要去的页,登录成功后跳回(见 login.component)。returnUrl 统一走 StorageService,
    // 不再塞 query:query 从没被消费、且扛不住整页跳门禁 reload。
    storageService.saveReturnUrl(state.url);
    router.navigate(['/login']);
    return false;
  }

  // 门禁态(强制两步验证但账号未绑定,只持有「仅绑定 token」):
  // 任何业务路由都拦回全屏绑定页,绑完才放行。
  if (storageService.isEnrollmentPending()) {
    router.navigate(['/two-factor-enroll']);
    return false;
  }

  return true;
};

