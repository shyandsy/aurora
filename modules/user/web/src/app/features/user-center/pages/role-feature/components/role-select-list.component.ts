import { Component, inject, OnInit, ChangeDetectorRef, output, input } from '@angular/core';
import { CommonModule } from '@angular/common';
import { UserCenterApi } from '../../../shared/services/user-center-api';
import { Role } from '../../../shared/models/role.dto';

@Component({
  selector: 'app-role-select-list',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './role-select-list.component.html',
  styleUrl: './role-select-list.component.css'
})
export class RoleSelectListComponent implements OnInit {
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);

  // Input: selected role ID
  selectedRoleId = input<number | null>(null);

  // Output: role selected event
  roleSelected = output<number>();

  roles: Role[] = [];
  isLoading = false;
  error: string | null = null;

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
        this.error = 'Failed to load roles';
        this.isLoading = false;
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * Handle role click
   */
  onRoleClick(role: Role): void {
    this.roleSelected.emit(role.id);
  }

  /**
   * Check if role is selected
   */
  isSelected(role: Role): boolean {
    return this.selectedRoleId() === role.id;
  }
}

