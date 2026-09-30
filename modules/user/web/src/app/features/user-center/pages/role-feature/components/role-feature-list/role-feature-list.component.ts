import { Component, inject, OnInit, ChangeDetectorRef, input, effect, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { UserCenterApi } from '../../../../shared/services/user-center-api';
import { RoleFeature } from '../../../../shared/models/role-feature.dto';
import { formatDateToBeijing } from '@common/utils/date.util';

@Component({
  selector: 'app-role-feature-list',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './role-feature-list.component.html',
  styleUrl: './role-feature-list.component.css'
})
export class RoleFeatureListComponent implements OnInit {
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);

  // Input: role ID to filter role features
  roleId = input.required<number>();

  // Input: refresh trigger
  refreshTrigger = input<number>(0);

  // Output: create event
  create = output<void>();

  roleFeatures: RoleFeature[] = [];
  isLoading = false;
  error: string | null = null;

  constructor() {
    // Reload when refresh trigger or roleId changes
    effect(() => {
      const trigger = this.refreshTrigger();
      const currentRoleId = this.roleId();
      if (trigger > 0 || currentRoleId > 0) {
        this.loadRoleFeatures();
      }
    });
  }

  ngOnInit(): void {
    this.loadRoleFeatures();
  }

  /**
   * Load role features from API
   */
  loadRoleFeatures(): void {
    const currentRoleId = this.roleId();
    if (!currentRoleId || currentRoleId <= 0) {
      this.roleFeatures = [];
      return;
    }

    this.isLoading = true;
    this.error = null;
    this.cdr.detectChanges();

    this.apiService.get<RoleFeature[]>(`/role-feature?roleId=${currentRoleId}`).subscribe({
      next: (data) => {
        this.roleFeatures = Array.isArray(data) ? data : [];
        this.isLoading = false;
        this.cdr.detectChanges();
      },
      error: (err) => {
        console.error('Error loading role features:', err);
        this.error = 'Failed to load role features';
        this.isLoading = false;
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * Handle create action
   */
  onCreate(): void {
    this.create.emit();
  }

  /**
   * Handle delete action
   */
  onDelete(roleFeature: RoleFeature): void {
    if (!confirm(`Are you sure you want to delete this role-feature association?`)) {
      return;
    }

    this.apiService.delete<{ message: string }>(`/role-feature/${roleFeature.id}`).subscribe({
      next: () => {
        // Reload role features after deletion
        this.loadRoleFeatures();
      },
      error: (err) => {
        console.error('Error deleting role feature:', err);
        this.error = 'Failed to delete role feature';
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * Format date to YYYY-MM-DD HH:mm:ss
   */
  formatDate(dateString: string): string {
    return formatDateToBeijing(dateString);
  }
}

