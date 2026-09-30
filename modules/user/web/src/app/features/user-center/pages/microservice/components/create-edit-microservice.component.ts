import { Component, inject, input, output, effect, ChangeDetectorRef, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormGroup, Validators, ReactiveFormsModule, FormArray, FormControl } from '@angular/forms';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { timeout, catchError, throwError } from 'rxjs';
import { TimeoutError } from 'rxjs';
import { UserCenterApi } from '../../../shared/services/user-center-api';
import { MicroserviceTokenFeature, CreateMicroserviceTokenFeatureRequest, UpdateMicroserviceTokenFeatureRequest } from '../../../shared/models/microservice.dto';
import { Feature } from '../../../shared/models/feature.dto';
import { extractErrorMessage } from '../../../../../shared/utils/error.util';
import { environment } from '../../../../../../environments/environment';

@Component({
  selector: 'app-create-edit-microservice',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, TranslateModule],
  templateUrl: './create-edit-microservice.component.html',
  styleUrl: './create-edit-microservice.component.css'
})
export class CreateEditMicroserviceComponent implements OnInit {
  private readonly formBuilder = inject(FormBuilder);
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly translate = inject(TranslateService);

  // Input: microservice to edit (null for create mode)
  microservice = input<MicroserviceTokenFeature | null>(null);

  // Output: close dialog event
  close = output<void>();
  // Output: microservice saved event
  saved = output<MicroserviceTokenFeature>();

  microserviceForm: FormGroup;
  isSubmitting = false;
  error: string | null = null;
  allFeatures: Feature[] = [];
  isLoadingFeatures = false;

  get isEditMode(): boolean {
    return this.microservice() !== null;
  }

  get formTitle(): string {
    return this.isEditMode
      ? this.translate.instant('microservice.edit')
      : this.translate.instant('microservice.create');
  }

  get featureListFormArray(): FormArray {
    return this.microserviceForm.get('featureList') as FormArray;
  }

  constructor() {
    this.microserviceForm = this.formBuilder.group({
      name: ['', [Validators.required, Validators.maxLength(255)]],
      description: ['', [Validators.required, Validators.maxLength(500)]],
      featureList: this.formBuilder.array([], [Validators.required, Validators.minLength(1)])
    });

    // Update form when microservice input changes
    effect(() => {
      const currentMicroservice = this.microservice();
      if (currentMicroservice) {
        // Clear existing form array
        while (this.featureListFormArray.length !== 0) {
          this.featureListFormArray.removeAt(0);
        }
        // Set selected features
        currentMicroservice.featureList.forEach(feature => {
          this.featureListFormArray.push(new FormControl(feature));
        });
        this.microserviceForm.patchValue({
          name: currentMicroservice.name,
          description: currentMicroservice.description
        });
      } else {
        // Clear form array
        while (this.featureListFormArray.length !== 0) {
          this.featureListFormArray.removeAt(0);
        }
        this.microserviceForm.reset({
          name: '',
          description: '',
          featureList: []
        });
      }
      this.error = null;
    });
  }

  ngOnInit(): void {
    this.loadFeatures();
  }

  loadFeatures(): void {
    this.isLoadingFeatures = true;
    this.apiService.get<Feature[]>('/feature').subscribe({
      next: (data) => {
        this.allFeatures = Array.isArray(data) ? data : [];
        this.isLoadingFeatures = false;
        this.cdr.detectChanges();
      },
      error: (err) => {
        console.error('Error loading features:', err);
        this.error = 'Failed to load features';
        this.isLoadingFeatures = false;
        this.cdr.detectChanges();
      }
    });
  }

  toggleFeature(featureName: string): void {
    const index = this.featureListFormArray.controls.findIndex(
      control => control.value === featureName
    );
    if (index >= 0) {
      this.featureListFormArray.removeAt(index);
    } else {
      this.featureListFormArray.push(new FormControl(featureName));
    }
  }

  isFeatureSelected(featureName: string): boolean {
    return this.featureListFormArray.controls.some(
      control => control.value === featureName
    );
  }

  onCancel(): void {
    while (this.featureListFormArray.length !== 0) {
      this.featureListFormArray.removeAt(0);
    }
    this.microserviceForm.reset({
      name: '',
      description: '',
      featureList: []
    });
    this.error = null;
    this.close.emit();
  }

  onSubmit(): void {
    if (this.microserviceForm.invalid || this.isSubmitting) {
      return;
    }

    this.isSubmitting = true;
    this.error = null;
    this.cdr.detectChanges();

    const formValue = this.microserviceForm.value;
    const featureList = this.featureListFormArray.controls.map(control => control.value);

    if (this.isEditMode) {
      const updateReq: UpdateMicroserviceTokenFeatureRequest = {
        name: formValue.name.trim(),
        description: formValue.description.trim(),
        featureList: featureList
      };

      this.apiService.updateMicroserviceTokenFeature(this.microservice()!.id, updateReq)
        .pipe(
          timeout(environment.apiTimeout),
          catchError((err) => {
            if (err instanceof TimeoutError) {
              this.error = this.translate.instant('common.timeout');
            } else {
              this.error = extractErrorMessage(err) || this.translate.instant('microservice.updateFailed');
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
            this.saved.emit(response);
            this.cdr.detectChanges();
          },
          error: () => {
            // Error already handled
          }
        });
    } else {
      const createReq: CreateMicroserviceTokenFeatureRequest = {
        name: formValue.name.trim(),
        description: formValue.description.trim(),
        featureList: featureList
      };

      this.apiService.createMicroserviceTokenFeature(createReq)
        .pipe(
          timeout(environment.apiTimeout),
          catchError((err) => {
            if (err instanceof TimeoutError) {
              this.error = this.translate.instant('common.timeout');
            } else {
              this.error = extractErrorMessage(err) || this.translate.instant('microservice.createFailed');
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
            this.saved.emit(response);
            this.cdr.detectChanges();
          },
          error: () => {
            // Error already handled
          }
        });
    }
  }

  getErrorMessage(fieldName: string): string {
    const control = this.microserviceForm.get(fieldName);
    if (control?.hasError('required')) {
      return this.translate.instant(`microservice.errors.${fieldName}Required`);
    }
    if (control?.hasError('maxlength')) {
      return this.translate.instant(`microservice.errors.${fieldName}MaxLength`);
    }
    if (control?.hasError('minlength')) {
      return this.translate.instant(`microservice.errors.${fieldName}MinLength`);
    }
    return '';
  }
}

