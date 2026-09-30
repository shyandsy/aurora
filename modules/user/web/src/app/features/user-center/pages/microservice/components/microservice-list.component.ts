import { Component, OnInit, inject, input, output, effect, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { timeout, catchError, throwError } from 'rxjs';
import { TimeoutError } from 'rxjs';
import { UserCenterApi } from '../../../shared/services/user-center-api';
import { MicroserviceTokenFeature } from '../../../shared/models/microservice.dto';
import { extractErrorMessage } from '../../../../../shared/utils/error.util';
import { environment } from '../../../../../../environments/environment';
import { ConfirmDialogComponent } from '../../../../../shared/components/confirm-dialog/confirm-dialog.component';
import { formatDate } from '@common/utils/date.util';

@Component({
  selector: 'app-microservice-list',
  standalone: true,
  imports: [CommonModule, TranslateModule, ConfirmDialogComponent],
  templateUrl: './microservice-list.component.html',
  styleUrl: './microservice-list.component.css'
})
export class MicroserviceListComponent implements OnInit {
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly translate = inject(TranslateService);

  // Input: refresh trigger
  refreshTrigger = input<number>(0);

  // Output: edit action
  editMicroservice = output<MicroserviceTokenFeature>();

  microservices: MicroserviceTokenFeature[] = [];
  isLoading = false;
  error: string | null = null;
  showDeleteConfirmDialog = false;
  microserviceToDelete: MicroserviceTokenFeature | null = null;

  constructor() {
    // Reload when refresh trigger changes
    effect(() => {
      const trigger = this.refreshTrigger();
      if (trigger > 0) {
        this.loadMicroservices();
      }
    });
  }

  ngOnInit(): void {
    this.loadMicroservices();
  }

  loadMicroservices(): void {
    this.isLoading = true;
    this.error = null;
    this.cdr.detectChanges();

    this.apiService.getMicroserviceTokenFeatures()
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
          return throwError(() => err);
        })
      )
      .subscribe({
        next: (data) => {
          this.microservices = data;
          this.isLoading = false;
          this.cdr.detectChanges();
        },
        error: () => {
          // Error already handled
        }
      });
  }

  onEdit(microservice: MicroserviceTokenFeature): void {
    this.editMicroservice.emit(microservice);
  }

  onDelete(microservice: MicroserviceTokenFeature): void {
    this.microserviceToDelete = microservice;
    this.showDeleteConfirmDialog = true;
  }

  confirmDelete(): void {
    if (!this.microserviceToDelete) {
      return;
    }

    const id = this.microserviceToDelete.id;
    this.apiService.deleteMicroserviceTokenFeature(id)
      .pipe(
        timeout(environment.apiTimeout),
        catchError((err) => {
          if (err instanceof TimeoutError) {
            this.error = this.translate.instant('common.timeout');
          } else {
            this.error = extractErrorMessage(err) || this.translate.instant('microservice.deleteFailed');
          }
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
          this.showDeleteConfirmDialog = false;
          this.microserviceToDelete = null;
          this.loadMicroservices();
        },
        error: () => {
          // Error already handled
        }
      });
  }

  cancelDelete(): void {
    this.showDeleteConfirmDialog = false;
    this.microserviceToDelete = null;
  }

  formatDate(dateString: string | null | undefined): string {
    return formatDate(dateString);
  }
}

