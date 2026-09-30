import { Component, inject, OnInit, ChangeDetectorRef, output, input, effect, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TranslateModule } from '@ngx-translate/core';
import { UserCenterApi } from '../../../shared/services/user-center-api';
import { User, PagingRequest } from '../../../shared/models/user.dto';
import { PagingResponse } from '@common/models/common.dto';
import { PaginationComponent } from '../../../../../shared/components/pagination/pagination.component';
import { ConfirmDialogComponent } from '../../../../../shared/components/confirm-dialog/confirm-dialog.component';

@Component({
  selector: 'app-user-list',
  standalone: true,
  imports: [CommonModule, PaginationComponent, TranslateModule, ConfirmDialogComponent],
  templateUrl: './user-list.component.html',
  styleUrl: './user-list.component.css'
})
export class UserListComponent implements OnInit {
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);

  // Input: refresh trigger
  refreshTrigger = input<number>(0);

  // Output: edit action
  editUser = output<User>();

  users: User[] = [];
  isLoading = false;
  error: string | null = null;

  // Delete confirmation dialog
  showDeleteConfirm = signal(false);
  userToDelete: User | null = null;

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
        this.loadUsers();
      }
    });
  }

  ngOnInit(): void {
    this.loadUsers();
  }

  /**
   * Load users from API
   */
  loadUsers(): void {
    this.isLoading = true;
    this.error = null;
    this.cdr.detectChanges();

    const pagingReq: PagingRequest = {
      page: this.currentPage,
      pageSize: this.pageSize
    };

    this.apiService.get<PagingResponse<User>>(`/user?page=${pagingReq.page}&pageSize=${pagingReq.pageSize}`).subscribe({
      next: (data) => {
        this.users = Array.isArray(data.items) ? data.items : [];
        this.total = data.total || 0;
        this.totalPages = data.totalPages || 0;
        this.hasNext = data.hasNext || false;
        this.hasPrev = data.hasPrev || false;
        this.isLoading = false;
        this.cdr.detectChanges();
      },
      error: (err) => {
        console.error('Error loading users:', err);
        this.error = err.error?.message || 'Failed to load users';
        this.isLoading = false;
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * Handle edit action
   */
  onEdit(user: User): void {
    this.editUser.emit(user);
  }

  /**
   * Handle delete action - show confirmation dialog
   */
  onDelete(user: User): void {
    this.userToDelete = user;
    this.showDeleteConfirm.set(true);
    this.cdr.detectChanges();
  }

  /**
   * Confirm delete action
   */
  onConfirmDelete(): void {
    if (!this.userToDelete) {
      return;
    }

    const userId = this.userToDelete.id;
    this.showDeleteConfirm.set(false);
    this.cdr.detectChanges();

    this.apiService.delete<{ message: string }>(`/user/${userId}`).subscribe({
      next: () => {
        // Reload users after deletion
        this.loadUsers();
        this.userToDelete = null;
      },
      error: (err) => {
        console.error('Error deleting user:', err);
        this.error = err.error?.message || 'Failed to delete user';
        this.userToDelete = null;
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * Cancel delete action
   */
  onCancelDelete(): void {
    this.showDeleteConfirm.set(false);
    this.userToDelete = null;
    this.cdr.detectChanges();
  }

  /**
   * Handle page change from pagination component
   */
  onPageChange(page: number): void {
    this.currentPage = page;
    this.loadUsers();
  }
}

