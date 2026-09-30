import { Component, EventEmitter, Input, Output, inject, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormGroup, Validators, ReactiveFormsModule, FormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { AuthService } from '../../../core/services/auth.service';
import { extractErrorMessage } from '../../utils/error.util';
import { otpauthToQrDataUrl } from '../../utils/qrcode.util';

type EnrollStep = 'password' | 'scan' | 'recovery';

/**
 * 两步验证(TOTP)绑定分步器 —— 三步走:重验密码 → 扫码/输码 → 一次性备用码。
 *
 * 复用于两处:
 * ①强制绑定全屏门禁页(two-factor-enroll);②设置页里的「启用两步验证」。
 * confirm 成功即用返回的正式 login 覆盖本地 token(强制场景下把「仅绑定 token」换成正式 token),
 * 再走备用码强提示保存,用户确认已保存后 emit completed,由外层决定跳转/刷新。
 *
 * 敏感字段:密码、验证码仅输入,绝不回显;备用码仅此一次展示(后端不再返回)。
 */
@Component({
  selector: 'app-totp-enroll',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, FormsModule, TranslateModule],
  templateUrl: './totp-enroll.component.html',
})
export class TotpEnrollComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly authService = inject(AuthService);
  private readonly cdr = inject(ChangeDetectorRef);

  /** 备用码已保存、绑定流程完成。外层据此清门禁 + 跳转,或刷新设置页状态。 */
  @Output() completed = new EventEmitter<void>();
  /** 用户中途取消(仅设置页启用场景用;强制门禁页不提供取消)。 */
  @Output() cancelled = new EventEmitter<void>();

  /** 是否显示「取消」按钮(设置页 true;强制门禁页 false)。 */
  @Input() showCancel = false;

  step: EnrollStep = 'password';
  isSubmitting = false;
  error: string | null = null;

  otpauthUri = '';
  secret = '';
  qrDataUrl = '';
  recoveryCodes: string[] = [];
  recoverySaved = false;

  passwordForm: FormGroup;
  codeForm: FormGroup;

  constructor() {
    this.passwordForm = this.formBuilder.group({
      password: ['', [Validators.required]],
    });
    this.codeForm = this.formBuilder.group({
      // 6 位 TOTP 或备用码,统一走这个框;不强制正好 6 位以兼容备用码格式。
      code: ['', [Validators.required, Validators.minLength(6)]],
    });
  }

  /** 外层可复用同一实例重开流程时用。 */
  reset(): void {
    this.step = 'password';
    this.isSubmitting = false;
    this.error = null;
    this.otpauthUri = '';
    this.secret = '';
    this.qrDataUrl = '';
    this.recoveryCodes = [];
    this.recoverySaved = false;
    this.passwordForm.reset();
    this.codeForm.reset();
    this.cdr.detectChanges();
  }

  submitPassword(): void {
    if (this.passwordForm.invalid || this.isSubmitting) {
      return;
    }
    this.isSubmitting = true;
    this.error = null;
    this.cdr.detectChanges();

    this.authService.setupTotp({ password: this.passwordForm.value.password }).subscribe({
      next: (resp) => {
        this.otpauthUri = resp.otpauthUri;
        this.secret = resp.secret;
        this.step = 'scan';
        this.isSubmitting = false;
        // 密码不再需要,清掉不留在内存态。
        this.passwordForm.reset();
        this.cdr.detectChanges();
        otpauthToQrDataUrl(this.otpauthUri)
          .then((url) => {
            this.qrDataUrl = url;
            this.cdr.detectChanges();
          })
          .catch(() => {
            // 二维码渲染失败不致命:仍可手动录入密钥。
            this.qrDataUrl = '';
            this.cdr.detectChanges();
          });
      },
      error: (err) => {
        this.error = extractErrorMessage(err);
        this.isSubmitting = false;
        this.cdr.detectChanges();
      },
    });
  }

  submitCode(): void {
    if (this.codeForm.invalid || this.isSubmitting) {
      return;
    }
    this.isSubmitting = true;
    this.error = null;
    this.cdr.detectChanges();

    this.authService.confirmTotp({ code: (this.codeForm.value.code || '').trim() }).subscribe({
      next: (resp) => {
        // 绑定成功即换正式 token(强制场景:仅绑定 token → 正式 token;设置场景:刷新 user.twoFactorEnabled)。
        if (resp.login && resp.login.accessToken) {
          this.authService.applyLoginResponse(resp.login);
        }
        this.recoveryCodes = resp.recoveryCodes || [];
        this.step = 'recovery';
        this.isSubmitting = false;
        this.codeForm.reset();
        this.cdr.detectChanges();
      },
      error: (err) => {
        this.error = extractErrorMessage(err);
        this.isSubmitting = false;
        this.cdr.detectChanges();
      },
    });
  }

  copyRecoveryCodes(): void {
    const text = this.recoveryCodes.join('\n');
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(text).catch(() => { /* 忽略:用户可手动选中复制 */ });
    }
  }

  finish(): void {
    if (!this.recoverySaved) {
      return;
    }
    this.completed.emit();
  }

  cancel(): void {
    this.cancelled.emit();
  }
}
