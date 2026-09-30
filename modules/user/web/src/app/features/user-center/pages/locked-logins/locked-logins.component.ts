import { Component, OnInit, inject, signal, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { timeout, catchError, throwError, TimeoutError } from 'rxjs';
import { UserCenterApi } from '../../shared/services/user-center-api';
import { LockedLoginItem } from '../../shared/models/rate-limit.dto';
import { extractErrorMessage } from '../../../../shared/utils/error.util';
import { environment } from '../../../../../environments/environment';
import { ConfirmDialogComponent } from '../../../../shared/components/confirm-dialog/confirm-dialog.component';
import { formatDateToBeijing } from '@common/utils/date.util';

/**
 * 被锁登录 / 登录限流(loginguard, user namespace)后台管理页。
 *
 * 列出当前被锁的 IP / 账号(维度、key、最近邮箱、原因、解除剩余时间、记录时刻),每行可手动解锁。
 * 数据一次性拉取(接口返回全量 items,无分页);解锁走确认弹窗 → 成功后刷新列表。
 * 门控在路由 + 壳 tab 上按 user.get(见 user-center.routes.ts / user-permission.component.ts)。
 */
@Component({
  selector: 'app-locked-logins',
  standalone: true,
  imports: [CommonModule, TranslateModule, ConfirmDialogComponent],
  templateUrl: './locked-logins.component.html',
  styleUrl: './locked-logins.component.css'
})
export class LockedLoginsComponent implements OnInit {
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly translate = inject(TranslateService);

  items = signal<LockedLoginItem[]>([]);
  isLoading = signal(false);
  error = signal<string | null>(null);

  // 解锁确认弹窗
  showUnlockConfirm = signal(false);
  unlocking = signal(false);
  private itemToUnlock: LockedLoginItem | null = null;

  ngOnInit(): void {
    this.loadLocked();
  }

  loadLocked(): void {
    this.isLoading.set(true);
    this.error.set(null);
    this.cdr.detectChanges();

    this.apiService.listLockedLogins()
      .pipe(
        timeout(environment.apiTimeout),
        catchError((err) => {
          this.showTransientError(err, 'common.loadFailed');
          this.isLoading.set(false);
          this.cdr.detectChanges();
          return throwError(() => err);
        })
      )
      .subscribe({
        next: (res) => {
          this.items.set(res.items || []);
          this.isLoading.set(false);
          this.cdr.detectChanges();
        },
        error: () => {
          // 已在 catchError 处理
        }
      });
  }

  onUnlock(item: LockedLoginItem): void {
    this.itemToUnlock = item;
    this.showUnlockConfirm.set(true);
    this.cdr.detectChanges();
  }

  onCancelUnlock(): void {
    this.showUnlockConfirm.set(false);
    this.itemToUnlock = null;
    this.cdr.detectChanges();
  }

  onConfirmUnlock(): void {
    const item = this.itemToUnlock;
    if (!item) {
      return;
    }
    this.showUnlockConfirm.set(false);
    this.unlocking.set(true);
    this.error.set(null);
    this.cdr.detectChanges();

    this.apiService.unlockLockedLogin({ scope: item.scope, key: item.key })
      .pipe(
        timeout(environment.apiTimeout),
        catchError((err) => {
          this.showTransientError(err, 'common.operationFailed');
          this.unlocking.set(false);
          this.itemToUnlock = null;
          this.cdr.detectChanges();
          return throwError(() => err);
        })
      )
      .subscribe({
        next: () => {
          this.unlocking.set(false);
          this.itemToUnlock = null;
          this.loadLocked();
        },
        error: () => {
          // 已在 catchError 处理
        }
      });
  }

  /** 弹窗里展示的目标标识(scope 标签 + key)。 */
  unlockTargetLabel(): string {
    const item = this.itemToUnlock;
    if (!item) {
      return '';
    }
    return `${this.scopeLabel(item.scope)}: ${item.key}`;
  }

  /** 维度标签。 */
  scopeLabel(scope: string): string {
    const key = scope === 'ip' ? 'lockedLogins.scope.ip' : 'lockedLogins.scope.account';
    return this.translate.instant(key);
  }

  /** 原因标签:已知取值走 i18n,未知取值原样展示(防后端新增值时空白)。 */
  reasonLabel(reason: string): string {
    const known = ['ip_locked', 'ip_hour_cap', 'acct_locked'];
    if (known.includes(reason)) {
      return this.translate.instant(`lockedLogins.reason.${reason}`);
    }
    return reason;
  }

  /** 解除剩余时间:秒 → HH:MM:SS;<=0 视为即将自动解除。 */
  formatRemaining(seconds: number): string {
    if (seconds == null || seconds <= 0) {
      return this.translate.instant('lockedLogins.autoUnlockSoon');
    }
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    const pad = (n: number) => String(n).padStart(2, '0');
    return h > 0 ? `${pad(h)}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`;
  }

  /** 记录时刻:unix 秒 → 北京时区展示。 */
  formatUpdatedAt(unixSeconds: number): string {
    if (!unixSeconds) {
      return '-';
    }
    return formatDateToBeijing(new Date(unixSeconds * 1000));
  }

  /** 统一的瞬时错误提示(自动隐藏),与本模块其它列表页一致。 */
  private showTransientError(err: unknown, fallbackKey: string): void {
    if (err instanceof TimeoutError) {
      this.error.set(this.translate.instant('common.timeout'));
    } else {
      this.error.set(extractErrorMessage(err) || this.translate.instant(fallbackKey));
    }
    setTimeout(() => {
      this.error.set(null);
      this.cdr.detectChanges();
    }, environment.errorMessageAutoHideTime);
  }
}
