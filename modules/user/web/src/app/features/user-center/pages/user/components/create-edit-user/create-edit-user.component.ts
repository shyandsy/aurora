import { Component, inject, input, output, effect, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormGroup, Validators, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { UserCenterApi } from '../../../../shared/services/user-center-api';
import { User, CreateUserRequest, UpdateUserRequest } from '../../../../shared/models/user.dto';
import { Role } from '../../../../shared/models/role.dto';

@Component({
  selector: 'app-create-edit-user',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, TranslateModule],
  templateUrl: './create-edit-user.component.html',
  styleUrl: './create-edit-user.component.css'
})
export class CreateEditUserComponent {
  private readonly formBuilder = inject(FormBuilder);
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly translate = inject(TranslateService);

  // Input: user to edit (null for create mode)
  user = input<User | null>(null);

  // Output: close dialog event
  close = output<void>();
  // Output: user saved event
  saved = output<User>();

  userForm: FormGroup;
  isSubmitting = false;
  // 密码默认可见(后台建号需能看/复制);切换按钮可隐藏。
  showPassword = true;
  error: string | null = null;

  roles: Role[] = [];
  isLoadingRoles = false;

  constructor() {
    this.userForm = this.formBuilder.group({
      email: ['', [Validators.required, Validators.email]],
      password: ['', []], // Optional for edit mode
      roleId: ['', [Validators.required]]
    });

    // Update form when user input changes
    effect(() => {
      const currentUser = this.user();
      if (currentUser) {
        this.userForm.patchValue({
          email: currentUser.email,
          roleId: currentUser.roleId.toString(),
          password: '' // Don't prefill password
        });
        // Remove password required for edit mode
        this.userForm.get('password')?.clearValidators();
      } else {
        this.userForm.reset();
        // Add password required for create mode
        this.userForm.get('password')?.setValidators([Validators.required]);
      }
      this.userForm.get('password')?.updateValueAndValidity();
      this.error = null;
      this.cdr.detectChanges();
    });

    // Load roles
    this.loadRoles();
  }

  /**
   * Check if in edit mode
   */
  get isEditMode(): boolean {
    return this.user() !== null;
  }

  /**
   * Get form title
   */
  get formTitle(): string {
    return this.isEditMode ? this.translate.instant('user.edit') : this.translate.instant('user.create');
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
   * Handle form submission
   */
  onSubmit(): void {
    if (this.userForm.invalid) {
      return;
    }

    this.isSubmitting = true;
    this.error = null;
    this.cdr.detectChanges();

    const formValue = this.userForm.value;
    const currentUser = this.user();

    if (currentUser) {
      // Update existing user
      const updateReq: UpdateUserRequest = {
        email: formValue.email,
        roleId: parseInt(formValue.roleId, 10)
      };

      // Only include password if provided
      if (formValue.password && formValue.password.trim() !== '') {
        updateReq.password = formValue.password;
      }

      this.apiService.put<User>(`/user/${currentUser.id}`, updateReq).subscribe({
        next: (user) => {
          this.saved.emit(user);
          this.isSubmitting = false;
          this.cdr.detectChanges();
        },
        error: (err) => {
          console.error('Error updating user:', err);
          this.error = err.error?.message || this.translate.instant('user.errors.updateFailed');
          this.isSubmitting = false;
          this.cdr.detectChanges();
        }
      });
    } else {
      // Create new user
      const createReq: CreateUserRequest = {
        email: formValue.email,
        password: formValue.password,
        roleId: parseInt(formValue.roleId, 10)
      };

      this.apiService.post<User>('/user', createReq).subscribe({
        next: (user) => {
          this.saved.emit(user);
          this.isSubmitting = false;
          this.cdr.detectChanges();
        },
        error: (err) => {
          console.error('Error creating user:', err);
          this.error = err.error?.message || this.translate.instant('user.errors.createFailed');
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
   * 随机生成密码:长度 8~10,保证至少各含 1 个大写字母、小写字母、数字,其余从三者合集随机,最后打乱。
   * 用 crypto.getRandomValues 取随机(比 Math.random 更均匀);生成后自动设为可见,方便复制。
   */
  generatePassword(): void {
    const upper = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
    const lower = 'abcdefghijklmnopqrstuvwxyz';
    const digit = '0123456789';
    const all = upper + lower + digit;
    const pick = (set: string) => set[Math.floor(this.rand() * set.length)];

    const len = 8 + Math.floor(this.rand() * 3); // 8 / 9 / 10
    const chars = [pick(upper), pick(lower), pick(digit)]; // 各类至少 1 个
    while (chars.length < len) {
      chars.push(pick(all));
    }
    // Fisher–Yates 打乱,避免「前三位固定是大写/小写/数字」的可预测性
    for (let i = chars.length - 1; i > 0; i--) {
      const j = Math.floor(this.rand() * (i + 1));
      [chars[i], chars[j]] = [chars[j], chars[i]];
    }
    this.userForm.get('password')?.setValue(chars.join(''));
    this.userForm.get('password')?.markAsDirty();
    this.showPassword = true; // 生成后显示,方便记录/复制
  }

  /** [0,1) 均匀随机,基于 crypto。 */
  private rand(): number {
    const buf = new Uint32Array(1);
    crypto.getRandomValues(buf);
    return buf[0] / 2 ** 32;
  }

  /**
   * Get form control error message
   */
  getErrorMessage(fieldName: string): string {
    const control = this.userForm.get(fieldName);
    if (control?.hasError('required')) {
      if (fieldName === 'email') {
        return this.translate.instant('user.errors.emailRequired');
      } else if (fieldName === 'password') {
        return this.translate.instant('user.errors.passwordRequired');
      } else if (fieldName === 'roleId') {
        return this.translate.instant('user.errors.roleRequired');
      }
      return this.translate.instant('common.error');
    }
    if (control?.hasError('email')) {
      return this.translate.instant('user.errors.emailInvalid');
    }
    return '';
  }
}

