import { Component, OnInit, inject, input, effect, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { timeout, catchError, throwError } from 'rxjs';
import { TimeoutError } from 'rxjs';
import { UserCenterApi } from '../../../shared/services/user-center-api';
import { MicroserviceTokenFeatureToken, GetMicroserviceTokenTokensRequest } from '../../../shared/models/microservice.dto';
import { PagingResponse } from '@common/models/common.dto';
import { extractErrorMessage } from '../../../../../shared/utils/error.util';
import { environment } from '../../../../../../environments/environment';
import { PaginationComponent } from '../../../../../shared/components/pagination/pagination.component';
import { formatDate } from '@common/utils/date.util';

@Component({
  selector: 'app-microservice-token-list',
  standalone: true,
  imports: [CommonModule, TranslateModule, PaginationComponent],
  templateUrl: './microservice-token-list.component.html',
  styleUrl: './microservice-token-list.component.css'
})
export class MicroserviceTokenListComponent implements OnInit {
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly translate = inject(TranslateService);

  // Input: refresh trigger
  refreshTrigger = input<number>(0);

  tokens: MicroserviceTokenFeatureToken[] = [];
  isLoading = false;
  error: string | null = null;

  // Pagination
  currentPage = 1;
  pageSize = 10;
  total = 0;
  totalPages = 0;
  hasNext = false;
  hasPrev = false;

  constructor() {
    // Reload when refresh trigger changes
    effect(() => {
      const trigger = this.refreshTrigger();
      if (trigger > 0) {
        this.loadTokens();
      }
    });
  }

  ngOnInit(): void {
    this.loadTokens();
  }

  loadTokens(): void {
    this.isLoading = true;
    this.error = null;
    this.cdr.detectChanges();

    const req: GetMicroserviceTokenTokensRequest = {
      page: this.currentPage,
      pageSize: this.pageSize
    };

    this.apiService.getMicroserviceTokenTokens(req)
      .pipe(
        timeout(environment.apiTimeout),
        catchError((err) => {
          if (err instanceof TimeoutError) {
            this.error = this.translate.instant('common.timeout');
          } else {
            this.error = extractErrorMessage(err) || this.translate.instant('common.loadFailed');
          }
          this.isLoading = false;
          this.cdr.detectChanges();
          setTimeout(() => {
            this.error = null;
            this.cdr.detectChanges();
          }, environment.errorMessageAutoHideTime);
          return throwError(() => err);
        })
      )
      .subscribe({
        next: (response: PagingResponse<MicroserviceTokenFeatureToken>) => {
          this.tokens = response.items || [];
          this.currentPage = response.page;
          this.pageSize = response.pageSize;
          this.total = response.total;
          this.totalPages = response.totalPages;
          this.hasNext = response.hasNext;
          this.hasPrev = response.hasPrev;
          this.isLoading = false;
          this.cdr.detectChanges();
        },
        error: () => {
          // Error already handled
        }
      });
  }

  onPageChange(page: number): void {
    this.currentPage = page;
    this.loadTokens();
  }

  formatDate(dateString: string | null | undefined): string {
    return formatDate(dateString);
  }

  copyToken(token: string): void {
    navigator.clipboard.writeText(token).then(() => {
      // Could show a toast notification here
    }).catch(err => {
      console.error('Failed to copy token:', err);
    });
  }

  enableToken(tokenId: number): void {
    this.isLoading = true;
    this.error = null;
    this.cdr.detectChanges();

    this.apiService.enableMicroserviceToken(tokenId)
      .pipe(
        timeout(environment.apiTimeout),
        catchError((err) => {
          if (err instanceof TimeoutError) {
            this.error = this.translate.instant('common.timeout');
          } else {
            this.error = extractErrorMessage(err) || this.translate.instant('common.operationFailed');
          }
          this.isLoading = false;
          this.cdr.detectChanges();
          setTimeout(() => {
            this.error = null;
            this.cdr.detectChanges();
          }, environment.errorMessageAutoHideTime);
          return throwError(() => err);
        })
      )
      .subscribe({
        next: () => {
          this.loadTokens();
        },
        error: () => {
          // Error already handled
        }
      });
  }

  disableToken(tokenId: number): void {
    this.isLoading = true;
    this.error = null;
    this.cdr.detectChanges();

    this.apiService.disableMicroserviceToken(tokenId)
      .pipe(
        timeout(environment.apiTimeout),
        catchError((err) => {
          if (err instanceof TimeoutError) {
            this.error = this.translate.instant('common.timeout');
          } else {
            this.error = extractErrorMessage(err) || this.translate.instant('common.operationFailed');
          }
          this.isLoading = false;
          this.cdr.detectChanges();
          setTimeout(() => {
            this.error = null;
            this.cdr.detectChanges();
          }, environment.errorMessageAutoHideTime);
          return throwError(() => err);
        })
      )
      .subscribe({
        next: () => {
          this.loadTokens();
        },
        error: () => {
          // Error already handled
        }
      });
  }

  isTokenEnabled(token: MicroserviceTokenFeatureToken): boolean {
    return token.status === 'ENABLED';
  }
}

