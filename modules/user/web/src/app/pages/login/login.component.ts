import { Component, inject, OnInit, ChangeDetectorRef } from '@angular/core';
import { FormBuilder, FormGroup, Validators, ReactiveFormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { AuthService } from '../../core/services/auth.service';
import { LoginRequest, LoginResponse } from '../../shared/models/auth.dto';
import { extractErrorMessage } from '../../shared/utils/error.util';
import { StorageService } from '../../core/services/storage.service';
import { OtpInputComponent } from '../../shared/components/otp-input/otp-input.component';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [ReactiveFormsModule, OtpInputComponent],
  templateUrl: './login.component.html',
  styleUrl: './login.component.css'
})
export class LoginComponent implements OnInit {
  private readonly formBuilder = inject(FormBuilder);
  private readonly authService = inject(AuthService);
  private readonly router = inject(Router);
  private readonly storageService = inject(StorageService);
  private readonly cdr = inject(ChangeDetectorRef);

  // 登录分两步:credentials(邮箱+密码)→(账号已开两步验证时)code(验证码)。
  step: 'credentials' | 'code' = 'credentials';
  pendingToken = '';

  loginForm: FormGroup;
  codeForm: FormGroup;
  isLoading = false;
  errorMessage: string | null = null;

  // 验证码输入方式:false=6 格数字方框(TOTP);true=文本框(10 位 hex 恢复码)。
  useRecoveryCode = false;

  // 验证码满 6 位纯数字后自动提交用的防抖计时器。
  private autoSubmitTimer: ReturnType<typeof setTimeout> | null = null;

  constructor() {
    this.loginForm = this.formBuilder.group({
      email: ['', [Validators.required, Validators.email]],
      password: ['', [Validators.required]]
    });
    this.codeForm = this.formBuilder.group({
      // 6 位 TOTP 或备用码,统一走这个框;不强制正好 6 位以兼容备用码格式。
      code: ['', [Validators.required, Validators.minLength(6)]]
    });
  }

  /**
   * Check if user is already authenticated on component initialization
   */
  ngOnInit(): void {
    // If user is already authenticated, redirect to home page
    if (this.authService.isAuthenticated()) {
      this.router.navigate(['/']);
    }

    // 验证码输满**正好 6 位纯数字**(= TOTP)就自动提交,省一次点确定。
    // 加 250ms 防抖:备用码是 10 位 hex,继续往下打会取消,避免"前 6 位是数字的备用码"被提前提交。
    this.codeForm.get('code')!.valueChanges.subscribe((value: string) => {
      this.clearAutoSubmit();
      if (/^\d{6}$/.test((value || '').trim()) && !this.isLoading) {
        this.autoSubmitTimer = setTimeout(() => this.onSubmitCode(), 250);
      }
    });
  }

  private clearAutoSubmit(): void {
    if (this.autoSubmitTimer) {
      clearTimeout(this.autoSubmitTimer);
      this.autoSubmitTimer = null;
    }
  }

  /**
   * Handle form submission
   */
  onSubmit(): void {
    if (this.loginForm.invalid) {
      return;
    }

    this.isLoading = true;
    this.errorMessage = null;

    const credentials: LoginRequest = {
      email: this.loginForm.value.email,
      password: this.loginForm.value.password
    };

    this.authService.login(credentials).subscribe({
      next: (response) => {
        // 账号已开两步验证:密码验过后不发正式 token,进第二步输验证码。
        if (response?.twoFactorRequired && response.pendingToken) {
          this.pendingToken = response.pendingToken;
          this.step = 'code';
          this.isLoading = false;
          this.codeForm.reset();
          this.cdr.detectChanges();
          return;
        }

        if (!response || !response.accessToken) {
          this.errorMessage = 'Invalid response from server: missing access token';
          this.isLoading = false;
          this.cdr.detectChanges();
          return;
        }

        this.applyAndRedirect(response);
      },
      error: (error) => {
        if (error.status === 401) {
          this.storageService.clear();
        }
        this.errorMessage = extractErrorMessage(error);
        this.isLoading = false;
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * 第二步:提交验证码(或备用码),凭 pendingToken 换正式 token。
   */
  onSubmitCode(): void {
    this.clearAutoSubmit();
    if (this.codeForm.invalid || this.isLoading) {
      return;
    }
    this.isLoading = true;
    this.errorMessage = null;
    this.cdr.detectChanges();

    this.authService.loginTwoFactor({
      pendingToken: this.pendingToken,
      code: (this.codeForm.value.code || '').trim()
    }).subscribe({
      next: (response) => {
        if (!response || !response.accessToken) {
          this.errorMessage = 'Invalid response from server: missing access token';
          this.isLoading = false;
          this.cdr.detectChanges();
          return;
        }
        this.applyAndRedirect(response);
      },
      error: (error) => {
        this.errorMessage = extractErrorMessage(error);
        this.isLoading = false;
        this.cdr.detectChanges();
      }
    });
  }

  /** 切换「验证码方框 ↔ 恢复码文本框」。清空已输入,避免两种模式串值。 */
  toggleRecovery(): void {
    this.useRecoveryCode = !this.useRecoveryCode;
    this.clearAutoSubmit();
    this.codeForm.reset();
    this.errorMessage = null;
  }

  /** 退回第一步(重填邮箱密码)。 */
  backToCredentials(): void {
    this.clearAutoSubmit();
    this.step = 'credentials';
    this.pendingToken = '';
    this.errorMessage = null;
    this.codeForm.reset();
    this.cdr.detectChanges();
  }

  /**
   * 落地登录态并跳转。强制两步验证但未绑定的账号(mustEnrollTwoFactor):
   * 此时 accessToken 是空权限的「仅绑定 token」,置门禁标志后引导去全屏绑定页。
   */
  private applyAndRedirect(response: LoginResponse): void {
    this.storageService.setAccessToken(response.accessToken);
    this.storageService.setRefreshToken(response.refreshToken);
    this.storageService.setUser(response.user || { features: response.features || [] });

    if (response.mustEnrollTwoFactor) {
      this.storageService.setEnrollmentPending(true);
      this.isLoading = false;
      this.router.navigate(['/two-factor-enroll']);
      return;
    }

    this.storageService.setEnrollmentPending(false);
    this.isLoading = false;
    // 跳回登录前用户本要去的页(守卫/拦截器暂存);没有则回首页。
    const target = this.storageService.takeReturnUrl();
    this.router.navigateByUrl(target || '/');
  }

  /**
   * Get form control error message
   */
  getErrorMessage(fieldName: string): string {
    const control = this.loginForm.get(fieldName);
    if (control?.hasError('required')) {
      return `${fieldName} is required`;
    }
    if (control?.hasError('email')) {
      return 'Please enter a valid email address';
    }
    return '';
  }
}
