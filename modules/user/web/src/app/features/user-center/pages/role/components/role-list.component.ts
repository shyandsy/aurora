import { Component, inject, OnInit, ChangeDetectorRef, output, input, effect, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TranslateModule } from '@ngx-translate/core';
import { UserCenterApi } from '../../../shared/services/user-center-api';
import { Role } from '../../../shared/models/role.dto';
import { ConfirmDialogComponent } from '../../../../../shared/components/confirm-dialog/confirm-dialog.component';
import { formatDateToBeijing } from '@common/utils/date.util';

@Component({
  selector: 'app-role-list',
  standalone: true,
  imports: [CommonModule, TranslateModule, ConfirmDialogComponent],
  templateUrl: './role-list.component.html',
  styleUrl: './role-list.component.css'
})
export class RoleListComponent implements OnInit {
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);

  // Input: refresh trigger
  refreshTrigger = input<number>(0);

  // Input: selected role ID (for highlighting)
  selectedRoleId = input<number | null>(null);

  // Output event for edit action
  editRole = output<Role>();

  // Output: role selected event (for row click)
  roleSelected = output<number>();

  roles: Role[] = [];
  isLoading = false;
  error: string | null = null;

  // Delete confirmation dialog
  showDeleteConfirm = signal(false);
  roleToDelete: Role | null = null;

  constructor() {
    // Reload when refresh trigger changes
    effect(() => {
      const trigger = this.refreshTrigger();
      if (trigger > 0) {
        this.loadRoles();
      }
    });
  }

  ngOnInit(): void {
    this.loadRoles();
  }

  /**
   * Load roles from API
   */
  loadRoles(): void {
    this.isLoading = true;
    this.error = null;
    this.cdr.detectChanges();

    this.apiService.get<Role[]>('/role').subscribe({
      next: (data) => {
        this.roles = Array.isArray(data) ? data : [];
        this.isLoading = false;
        this.cdr.detectChanges();
      },
      error: (err) => {
        console.error('Error loading roles:', err);
        this.error = err.error?.message || 'Failed to load roles';
        this.isLoading = false;
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * Handle edit action
   */
  onEdit(role: Role, event?: Event): void {
    if (event) {
      event.stopPropagation(); // Prevent row click when clicking edit button
    }
    this.editRole.emit(role);
  }

  /**
   * Handle row click (select role)
   */
  onRowClick(role: Role): void {
    this.roleSelected.emit(role.id);
  }

  /**
   * Handle delete action - show confirmation dialog
   */
  onDelete(role: Role, event?: Event): void {
    if (event) {
      event.stopPropagation(); // Prevent row click when clicking delete button
    }
    this.roleToDelete = role;
    this.showDeleteConfirm.set(true);
    this.cdr.detectChanges();
  }

  /**
   * Confirm delete action
   */
  onConfirmDelete(): void {
    if (!this.roleToDelete) {
      return;
    }

    const roleId = this.roleToDelete.id;
    this.showDeleteConfirm.set(false);
    this.cdr.detectChanges();

    this.apiService.delete<{ message: string }>(`/role/${roleId}`).subscribe({
      next: () => {
        // Emit roleDeleted event if the deleted role was selected
        if (this.selectedRoleId() === roleId) {
          this.roleSelected.emit(-1); // Emit -1 to indicate deselection
        }
        // Reload roles after deletion
        this.loadRoles();
        this.roleToDelete = null;
      },
      error: (err) => {
        console.error('Error deleting role:', err);
        this.error = err.error?.message || 'Failed to delete role';
        this.roleToDelete = null;
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * Cancel delete action
   */
  onCancelDelete(): void {
    this.showDeleteConfirm.set(false);
    this.roleToDelete = null;
    this.cdr.detectChanges();
  }

  /**
   * Format date to YYYY-MM-DD HH:mm:ss
   */
  formatDate(dateString: string): string {
    return formatDateToBeijing(dateString);
  }
}

