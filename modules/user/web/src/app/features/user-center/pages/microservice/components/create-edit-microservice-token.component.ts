import { Component, inject, output, ChangeDetectorRef, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormGroup, Validators, ReactiveFormsModule, FormArray, FormControl } from '@angular/forms';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { timeout, catchError, throwError } from 'rxjs';
import { TimeoutError } from 'rxjs';
import { UserCenterApi } from '../../../shared/services/user-center-api';
import { MicroserviceTokenFeature, IssueMicroserviceTokenRequest } from '../../../shared/models/microservice.dto';
import { extractErrorMessage } from '../../../../../shared/utils/error.util';
import { environment } from '../../../../../../environments/environment';

@Component({
  selector: 'app-create-edit-microservice-token',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, TranslateModule],
  templateUrl: './create-edit-microservice-token.component.html',
  styleUrl: './create-edit-microservice-token.component.css'
})
export class CreateEditMicroserviceTokenComponent implements OnInit {
  private readonly formBuilder = inject(FormBuilder);
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly translate = inject(TranslateService);

  // Output: close dialog event
  close = output<void>();
  // Output: token saved event
  saved = output<void>();

  tokenForm: FormGroup;
  isSubmitting = false;
  error: string | null = null;
  allMicroservices: MicroserviceTokenFeature[] = [];
  isLoadingMicroservices = false;
  generatedToken: string | null = null;

  get microserviceIdsFormArray(): FormArray {
    return this.tokenForm.get('microserviceFeatureIds') as FormArray;
  }

  constructor() {
    this.tokenForm = this.formBuilder.group({
      microserviceFeatureIds: this.formBuilder.array([], [Validators.required, Validators.minLength(1)])
    });
  }

  ngOnInit(): void {
    this.loadMicroservices();
  }

  loadMicroservices(): void {
    this.isLoadingMicroservices = true;
    this.apiService.getMicroserviceTokenFeatures().subscribe({
      next: (data) => {
        this.allMicroservices = Array.isArray(data) ? data : [];
        this.isLoadingMicroservices = false;
        this.cdr.detectChanges();
      },
      error: (err) => {
        console.error('Error loading microservices:', err);
        this.error = 'Failed to load microservices';
        this.isLoadingMicroservices = false;
        this.cdr.detectChanges();
      }
    });
  }

  toggleMicroservice(microserviceId: number): void {
    const index = this.microserviceIdsFormArray.controls.findIndex(
      control => control.value === microserviceId
    );
    if (index >= 0) {
      this.microserviceIdsFormArray.removeAt(index);
    } else {
      this.microserviceIdsFormArray.push(new FormControl(microserviceId));
    }
  }

  isMicroserviceSelected(microserviceId: number): boolean {
    return this.microserviceIdsFormArray.controls.some(
      control => control.value === microserviceId
    );
  }

  onCancel(): void {
    while (this.microserviceIdsFormArray.length !== 0) {
      this.microserviceIdsFormArray.removeAt(0);
    }
    this.tokenForm.reset({
      microserviceFeatureIds: []
    });
    this.error = null;
    this.generatedToken = null;
    this.close.emit();
  }

  onSubmit(): void {
    if (this.tokenForm.invalid || this.isSubmitting) {
      return;
    }

    this.isSubmitting = true;
    this.error = null;
    this.generatedToken = null;
    this.cdr.detectChanges();

    const microserviceFeatureIds = this.microserviceIdsFormArray.controls.map(control => control.value);

    const req: IssueMicroserviceTokenRequest = {
      microserviceFeatureIds: microserviceFeatureIds
    };

    this.apiService.issueMicroserviceToken(req)
      .pipe(
        timeout(environment.apiTimeout),
        catchError((err) => {
          if (err instanceof TimeoutError) {
            this.error = this.translate.instant('common.timeout');
          } else {
            this.error = extractErrorMessage(err) || this.translate.instant('microservice.token.createFailed');
          }
          this.isSubmitting = false;
          this.cdr.detectChanges();
          setTimeout(() => {
            this.error = null;
            this.cdr.detectChanges();
          }, environment.errorMessageAutoHideTime);
          return throwError(() => err);
        })
      )
      .subscribe({
        next: (response) => {
          this.isSubmitting = false;
          this.generatedToken = response.token;
          this.cdr.detectChanges();
          // Emit saved event immediately to refresh the list
          this.saved.emit();
          // Auto close after 3 seconds
          setTimeout(() => {
            this.close.emit();
          }, 3000);
        },
        error: () => {
          // Error already handled
        }
      });
  }

  copyToken(): void {
    if (this.generatedToken) {
      navigator.clipboard.writeText(this.generatedToken).then(() => {
        // Could show a toast notification here
      }).catch(err => {
        console.error('Failed to copy token:', err);
      });
    }
  }

  getErrorMessage(fieldName: string): string {
    const control = this.tokenForm.get(fieldName);
    if (control?.hasError('required')) {
      return this.translate.instant(`microservice.token.errors.${fieldName}Required`);
    }
    if (control?.hasError('minlength')) {
      return this.translate.instant(`microservice.token.errors.${fieldName}MinLength`);
    }
    return '';
  }
}

