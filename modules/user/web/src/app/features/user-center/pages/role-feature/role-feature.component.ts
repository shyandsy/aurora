import { Component, signal } from '@angular/core';
import { TranslateModule } from '@ngx-translate/core';
import { RoleListComponent } from '../role/components/role-list.component';
import { CreateEditRoleComponent } from '../role/components/create-edit-role/create-edit-role.component';
import { FeatureCheckboxListComponent } from './components/feature-checkbox-list/feature-checkbox-list.component';
import { PagePermissionListComponent } from './components/page-permission-list/page-permission-list.component';
import { Role } from '../../shared/models/role.dto';

@Component({
  selector: 'app-role-feature',
  standalone: true,
  imports: [RoleListComponent, CreateEditRoleComponent, FeatureCheckboxListComponent, PagePermissionListComponent, TranslateModule],
  templateUrl: './role-feature.component.html',
  styleUrl: './role-feature.component.css'
})
export class RoleFeatureComponent {
  selectedRoleId = signal<number | null>(null);
  refreshTrigger = signal(0);
  roleListRefreshTrigger = signal(0);
  showCreateEditDialog = signal(false);
  editingRole = signal<Role | null>(null);
  featureFilter = signal<string>('');
  // 授权视图:'page' = 页面级 view/操作分档(推荐);'advanced' = 直接勾选单个 feature。
  authMode = signal<'page' | 'advanced'>('page');

  /**
   * Handle role selection from role list
   */
  onRoleSelect(roleId: number): void {
    if (roleId === -1) {
      // Deselect role
      this.selectedRoleId.set(null);
    } else {
      this.selectedRoleId.set(roleId);
    }
    // Trigger refresh of feature checkbox list
    this.refreshTrigger.set(this.refreshTrigger() + 1);
  }

  /**
   * Handle create button click
   */
  onCreate(): void {
    this.editingRole.set(null);
    this.showCreateEditDialog.set(true);
  }

  /**
   * Handle edit event from role list
   */
  onEdit(role: Role): void {
    this.editingRole.set(role);
    this.showCreateEditDialog.set(true);
  }

  /**
   * Handle role saved event
   */
  onRoleSaved(): void {
    this.showCreateEditDialog.set(false);
    this.editingRole.set(null);
    // Trigger refresh of the role list
    this.roleListRefreshTrigger.set(this.roleListRefreshTrigger() + 1);
  }

  /**
   * Handle dialog close
   */
  onDialogClose(): void {
    this.showCreateEditDialog.set(false);
    this.editingRole.set(null);
  }

  /**
   * Handle feature update (triggered by checkbox change)
   */
  onFeatureUpdated(): void {
    // Trigger refresh to update checkbox states
    this.refreshTrigger.set(this.refreshTrigger() + 1);
  }
}

