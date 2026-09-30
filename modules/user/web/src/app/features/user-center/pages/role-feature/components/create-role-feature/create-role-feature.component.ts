import { Component, inject, input, output, effect, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormGroup, Validators, ReactiveFormsModule } from '@angular/forms';
import { UserCenterApi } from '../../../../shared/services/user-center-api';
import { RoleFeature, CreateRoleFeatureRequest } from '../../../../shared/models/role-feature.dto';
import { Role } from '../../../../shared/models/role.dto';
import { Feature } from '../../../../shared/models/feature.dto';

@Component({
  selector: 'app-create-role-feature',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule],
  templateUrl: './create-role-feature.component.html',
  styleUrl: './create-role-feature.component.css'
})
export class CreateRoleFeatureComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);

  // Input: default role ID (if provided)
  defaultRoleId = input<number | null>(null);

  // Output: close dialog event
  close = output<void>();
  // Output: role feature saved event
  saved = output<RoleFeature>();

  roleFeatureForm: FormGroup;
  isSubmitting = false;
  error: string | null = null;

  roles: Role[] = [];
  features: Feature[] = [];
  isLoadingRoles = false;
  isLoadingFeatures = false;

  constructor() {
    this.roleFeatureForm = this.formBuilder.group({
      roleId: ['', [Validators.required]],
      featureId: ['', [Validators.required]]
    });

    // Set default role ID if provided
    effect(() => {
      const roleId = this.defaultRoleId();
      if (roleId) {
        this.roleFeatureForm.patchValue({ roleId: roleId.toString() });
      }
      this.cdr.detectChanges();
    });

    // Load roles and features
    this.loadRoles();
    this.loadFeatures();
  }

  /**
   * Load roles from API
   */
  loadRoles(): void {
    this.isLoadingRoles = true;
    this.apiService.get<Role[]>('/role').subscribe({
      next: (data) => {
        this.roles = Array.isArray(data) ? data : [];
        this.isLoadingRoles = false;
        this.cdr.detectChanges();
      },
      error: (err) => {
        console.error('Error loading roles:', err);
        this.isLoadingRoles = false;
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * Load features from API
   */
  loadFeatures(): void {
    this.isLoadingFeatures = true;
    this.apiService.get<Feature[]>('/feature').subscribe({
      next: (data) => {
        this.features = Array.isArray(data) ? data : [];
        this.isLoadingFeatures = false;
        this.cdr.detectChanges();
      },
      error: (err) => {
        console.error('Error loading features:', err);
        this.isLoadingFeatures = false;
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * Handle form submission
   */
  onSubmit(): void {
    if (this.roleFeatureForm.invalid) {
      return;
    }

    this.isSubmitting = true;
    this.error = null;
    this.cdr.detectChanges();

    const formValue = this.roleFeatureForm.value;
    const createReq: CreateRoleFeatureRequest = {
      roleId: parseInt(formValue.roleId, 10),
      featureId: parseInt(formValue.featureId, 10)
    };

    this.apiService.post<RoleFeature>('/role-feature', createReq).subscribe({
      next: (roleFeature) => {
        this.saved.emit(roleFeature);
        this.isSubmitting = false;
        this.cdr.detectChanges();
      },
      error: (err) => {
        console.error('Error creating role feature:', err);
        this.error = 'Failed to create role feature';
        this.isSubmitting = false;
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * Handle cancel
   */
  onCancel(): void {
    this.close.emit();
  }

  /**
   * Get form control error message
   */
  getErrorMessage(fieldName: string): string {
    const control = this.roleFeatureForm.get(fieldName);
    if (control?.hasError('required')) {
      return `${fieldName} is required`;
    }
    return '';
  }
}

