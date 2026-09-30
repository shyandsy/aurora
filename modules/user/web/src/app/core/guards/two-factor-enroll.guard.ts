import { inject } from '@angular/core';
import { Router, CanActivateFn } from '@angular/router';
import { StorageService } from '../services/storage.service';

/**
 * 全屏强制绑定页的守卫:只有「已登录 + 处于未绑定门禁态」才允许进入。
 * - 未登录 → 回登录页;
 * - 已登录但没有门禁标志(不需要强制绑定 / 已绑定)→ 回首页,避免误入。
 */
export const twoFactorEnrollGuard: CanActivateFn = () => {
  const storageService = inject(StorageService);
  const router = inject(Router);

  if (!storageService.isAuthenticated()) {
    router.navigate(['/login']);
    return false;
  }

  if (!storageService.isEnrollmentPending()) {
    router.navigate(['/']);
    return false;
  }

  return true;
};
