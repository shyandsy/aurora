import { inject } from '@angular/core';
import { Router, CanActivateFn } from '@angular/router';
import { AuthService } from '../services/auth.service';
import { StorageService } from '../services/storage.service';

/**
 * Feature(权限)守卫。用法:路由上配 `data: { feature: 'commission.loss.get' }`。
 *
 * 定位:这是**体验层**的守卫,不是安全边界 —— 真正的鉴权在后端中间件
 * (JWTAuthMiddleware(app, "commission.loss.get")),前端只是别把用不了的页面摆出来。
 *
 * features 来源:AuthService.getUserFeatures() 从**当前 access token** 权威解出(不是 localStorage
 * 快照)。token 一定带着该角色的 features,故「解出为空」= 这个会话真的没有任何 feature(零权限角色 /
 * 仅绑定态 token),此时该拦就拦 —— **不再「空就放行」**(那会让 ui.menu.* 门形同虚设)。
 * 未登录由上面的 isAuthenticated 分支拦掉;登录/绑定 TOTP 等页面本就没配 featureGuard,不受影响。
 */
export const featureGuard: CanActivateFn = (route, state) => {
  const authService = inject(AuthService);
  const storageService = inject(StorageService);
  const router = inject(Router);

  // 未登录先走登录页(与 authGuard 行为一致,防止路由上漏配 authGuard 时裸奔)
  if (!storageService.isAuthenticated()) {
    // 存下用户本要去的页,登录成功后跳回(见 login.component)。returnUrl 统一走 StorageService,不再塞 query。
    storageService.saveReturnUrl(state.url);
    router.navigate(['/login']);
    return false;
  }

  const required = route.data?.['feature'] as string | undefined;
  if (!required) {
    return true;
  }

  // hasFeature 内含 '*' 超管通配:持有 '*' 则任何页面(含将来新增)都放行。
  if (authService.hasFeature(required)) {
    return true;
  }

  router.navigate(['/']);
  return false;
};
