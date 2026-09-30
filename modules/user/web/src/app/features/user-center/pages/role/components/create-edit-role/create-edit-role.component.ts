import { Component, inject, input, output, effect, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormGroup, Validators, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { UserCenterApi } from '../../../../shared/services/user-center-api';
import { Role, CreateRoleRequest, UpdateRoleRequest } from '../../../../shared/models/role.dto';

@Component({
  selector: 'app-create-edit-role',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, TranslateModule],
  templateUrl: './create-edit-role.component.html',
  styleUrl: './create-edit-role.component.css'
})
export class CreateEditRoleComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly translate = inject(TranslateService);

  // Input: role to edit (null for create mode)
  role = input<Role | null>(null);

  // Output: close dialog event
  close = output<void>();
  // Output: role saved event
  saved = output<Role>();

  roleForm: FormGroup;
  isSubmitting = false;
  error: string | null = null;

  constructor() {
    this.roleForm = this.formBuilder.group({
      name: ['', [Validators.required, Validators.minLength(1)]]
    });

    // Update form when role input changes
    effect(() => {
      const currentRole = this.role();
      if (currentRole) {
        this.roleForm.patchValue({
          name: currentRole.name
        });
      } else {
        this.roleForm.reset();
      }
      this.error = null;
      this.cdr.detectChanges();
    });
  }

  /**
   * Check if in edit mode
   */
  get isEditMode(): boolean {
    return this.role() !== null;
  }

  /**
   * Get form title
   */
  get formTitle(): string {
    return this.isEditMode ? this.translate.instant('role.edit') : this.translate.instant('role.create');
  }

  /**
   * Handle form submission
   */
  onSubmit(): void {
    if (this.roleForm.invalid) {
      return;
    }

    this.isSubmitting = true;
    this.error = null;
    this.cdr.detectChanges();

    const formValue = this.roleForm.value;
    const currentRole = this.role();

    if (currentRole) {
      // Update existing role
      const updateReq: UpdateRoleRequest = {
        name: formValue.name
      };

      this.apiService.put<Role>(`/role/${currentRole.id}`, updateReq).subscribe({
        next: (role) => {
          this.saved.emit(role);
          this.isSubmitting = false;
          this.cdr.detectChanges();
        },
        error: (err) => {
          console.error('Error updating role:', err);
          this.error = err.error?.message || this.translate.instant('role.errors.updateFailed');
          this.isSubmitting = false;
          this.cdr.detectChanges();
        }
      });
    } else {
      // Create new role
      const createReq: CreateRoleRequest = {
        name: formValue.name
      };

      this.apiService.post<Role>('/role', createReq).subscribe({
        next: (role) => {
          this.saved.emit(role);
          this.isSubmitting = false;
          this.cdr.detectChanges();
        },
        error: (err) => {
          console.error('Error creating role:', err);
          this.error = err.error?.message || this.translate.instant('role.errors.createFailed');
          this.isSubmitting = false;
          this.cdr.detectChanges();
        }
      });
    }
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
    const control = this.roleForm.get(fieldName);
    if (control?.hasError('required')) {
      return this.translate.instant('role.errors.nameRequired');
    }
    if (control?.hasError('minlength')) {
      return this.translate.instant('role.errors.nameMinLength');
    }
    return '';
  }
}

