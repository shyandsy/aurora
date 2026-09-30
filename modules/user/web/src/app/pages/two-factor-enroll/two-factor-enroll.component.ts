import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { Router } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { TotpEnrollComponent } from '../../shared/components/totp-enroll/totp-enroll.component';
import { StorageService } from '../../core/services/storage.service';

/**
 * 强制两步验证全屏门禁页。
 *
 * 系统开启「强制两步验证」而当前账号尚未绑定时,登录只拿到空权限的「仅绑定 token」,
 * 路由守卫把一切业务路由拦到这里。用户必须完成绑定(totp-enroll 内部 confirm 返回正式 token)
 * 才清门禁标志、放行进系统。这里不提供取消/跳过 —— 不绑定就出不去。
 */
@Component({
  selector: 'app-two-factor-enroll',
  standalone: true,
  imports: [CommonModule, TranslateModule, TotpEnrollComponent],
  templateUrl: './two-factor-enroll.component.html',
})
export class TwoFactorEnrollComponent {
  private readonly router = inject(Router);
  private readonly storageService = inject(StorageService);

  onCompleted(): void {
    // totp-enroll 已用 confirm 返回的正式 login 覆盖了本地 token,这里只需清门禁标志 + 进系统。
    this.storageService.setEnrollmentPending(false);
    this.router.navigate(['/']);
  }
}
