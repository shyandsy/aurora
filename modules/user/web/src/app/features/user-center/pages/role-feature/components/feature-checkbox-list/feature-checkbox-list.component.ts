import { Component, inject, OnInit, ChangeDetectorRef, input, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { forkJoin } from 'rxjs';
import { UserCenterApi } from '../../../../shared/services/user-center-api';
import { Feature } from '../../../../shared/models/feature.dto';
import { RoleFeature } from '../../../../shared/models/role-feature.dto';

@Component({
  selector: 'app-feature-checkbox-list',
  standalone: true,
  imports: [CommonModule, FormsModule, TranslateModule],
  templateUrl: './feature-checkbox-list.component.html',
  styleUrl: './feature-checkbox-list.component.css'
})
export class FeatureCheckboxListComponent implements OnInit {
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);

  // Input: selected role ID
  selectedRoleId = input<number | null>(null);

  // Input: refresh trigger
  refreshTrigger = input<number>(0);

  // Input: filter text for features
  filterText = input<string>('');

  features: Feature[] = [];
  roleFeatures: RoleFeature[] = [];
  isLoading = false;
  isLoadingRoleFeatures = false;
  error: string | null = null;
  updatingFeatureId: number | null = null;
  isBatchUpdating = false;

  constructor() {
    // Reload when refresh trigger or selectedRoleId changes
    effect(() => {
      const trigger = this.refreshTrigger();
      const roleId = this.selectedRoleId();
      if (trigger > 0 || (roleId && roleId > 0)) {
        this.loadRoleFeatures();
      }
    });
  }

  ngOnInit(): void {
    this.loadFeatures();
  }

  /**
   * Load all features from API
   */
  loadFeatures(): void {
    this.isLoading = true;
    this.error = null;
    this.cdr.detectChanges();

    this.apiService.get<Feature[]>('/feature').subscribe({
      next: (data) => {
        this.features = Array.isArray(data) ? data : [];
        this.isLoading = false;
        this.cdr.detectChanges();
        // Load role features after features are loaded
        if (this.selectedRoleId()) {
          this.loadRoleFeatures();
        }
      },
      error: (err) => {
        console.error('Error loading features:', err);
        this.error = 'Failed to load features';
        this.isLoading = false;
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * Load role features for selected role
   */
  loadRoleFeatures(): void {
    const roleId = this.selectedRoleId();
    if (!roleId || roleId <= 0) {
      this.roleFeatures = [];
      return;
    }

    this.isLoadingRoleFeatures = true;
    this.cdr.detectChanges();

    this.apiService.get<RoleFeature[]>(`/role-feature?roleId=${roleId}`).subscribe({
      next: (data) => {
        this.roleFeatures = Array.isArray(data) ? data : [];
        this.isLoadingRoleFeatures = false;
        this.cdr.detectChanges();
      },
      error: (err) => {
        console.error('Error loading role features:', err);
        this.isLoadingRoleFeatures = false;
        this.cdr.detectChanges();
      }
    });
  }

  /**
   * Check if feature is associated with selected role
   */
  isFeatureSelected(feature: Feature): boolean {
    return this.roleFeatures.some(rf => rf.featureId === feature.id);
  }

  /**
   * Handle checkbox change
   */
  onFeatureToggle(feature: Feature, checked: boolean): void {
    const roleId = this.selectedRoleId();
    if (!roleId || roleId <= 0) {
      return;
    }

    this.updatingFeatureId = feature.id;
    this.cdr.detectChanges();

    if (checked) {
      // Create role-feature association
      this.apiService.post<RoleFeature>('/role-feature', {
        roleId: roleId,
        featureId: feature.id
      }).subscribe({
        next: () => {
          // Reload role features
          this.loadRoleFeatures();
          this.updatingFeatureId = null;
          this.cdr.detectChanges();
        },
        error: (err) => {
          console.error('Error creating role feature:', err);
          this.error = 'Failed to create role feature';
          this.updatingFeatureId = null;
          this.cdr.detectChanges();
        }
      });
    } else {
      // Delete role-feature association
      const roleFeature = this.roleFeatures.find(rf => rf.featureId === feature.id);
      if (roleFeature) {
        this.apiService.delete<{ message: string }>(`/role-feature/${roleFeature.id}`).subscribe({
          next: () => {
            // Reload role features
            this.loadRoleFeatures();
            this.updatingFeatureId = null;
            this.cdr.detectChanges();
          },
          error: (err) => {
            console.error('Error deleting role feature:', err);
            this.error = 'Failed to delete role feature';
            this.updatingFeatureId = null;
            this.cdr.detectChanges();
          }
        });
      }
    }
  }

  /**
   * Check if feature is being updated
   */
  isUpdating(feature: Feature): boolean {
    return this.updatingFeatureId === feature.id;
  }

  /**
   * Get filtered features based on filter text.
   * 隐藏 ui.page.* / ui.menu.*:前者是页面级 view/operate 的内部入口、后者由页面档位自动带上,
   * 两者都归「页面权限」表管,不在这个「高级:直接授单个 feature」列表里重复暴露(否则双重管理会打架)。
   * 这里仍保留业务 api feature 与 ui.component.*(如 dashboard 统计、孤儿/高危 feature),供细粒度直接授予。
   */
  getFilteredFeatures(): Feature[] {
    const base = this.features.filter(f =>
      f.name !== '*' && !f.name.startsWith('ui.page.') && !f.name.startsWith('ui.menu.')
    );
    const filter = this.filterText().toLowerCase().trim();
    if (!filter) {
      return base;
    }
    return base.filter(feature =>
      feature.name.toLowerCase().includes(filter) ||
      feature.id.toString().includes(filter)
    );
  }

  /** 是否可批量操作(选了角色、当前没在批量/单勾中)。 */
  canBatch(): boolean {
    const roleId = this.selectedRoleId();
    return !!roleId && roleId > 0 && !this.isBatchUpdating && this.updatingFeatureId === null;
  }

  /** 全选:把当前筛选后可见、且尚未勾选的 feature 逐个加给角色(尊重筛选)。 */
  selectAllVisible(): void {
    const roleId = this.selectedRoleId();
    if (!roleId || roleId <= 0 || this.isBatchUpdating) {
      return;
    }
    const toAdd = this.getFilteredFeatures().filter(f => !this.isFeatureSelected(f));
    if (toAdd.length === 0) {
      return;
    }
    this.isBatchUpdating = true;
    this.error = null;
    this.cdr.detectChanges();
    forkJoin(toAdd.map(f =>
      this.apiService.post<RoleFeature>('/role-feature', { roleId, featureId: f.id })
    )).subscribe({
      next: () => this.finishBatch(),
      error: (err) => {
        console.error('Error batch adding role features:', err);
        this.error = 'Failed to update role features';
        this.finishBatch();
      }
    });
  }

  /** 全不选:把当前筛选后可见、且已勾选的 feature 逐个从角色移除(尊重筛选)。 */
  deselectAllVisible(): void {
    const roleId = this.selectedRoleId();
    if (!roleId || roleId <= 0 || this.isBatchUpdating) {
      return;
    }
    const toRemove = this.getFilteredFeatures()
      .map(f => this.roleFeatures.find(rf => rf.featureId === f.id))
      .filter((rf): rf is RoleFeature => !!rf);
    if (toRemove.length === 0) {
      return;
    }
    this.isBatchUpdating = true;
    this.error = null;
    this.cdr.detectChanges();
    forkJoin(toRemove.map(rf =>
      this.apiService.delete<{ message: string }>(`/role-feature/${rf.id}`)
    )).subscribe({
      next: () => this.finishBatch(),
      error: (err) => {
        console.error('Error batch removing role features:', err);
        this.error = 'Failed to update role features';
        this.finishBatch();
      }
    });
  }

  private finishBatch(): void {
    this.isBatchUpdating = false;
    this.loadRoleFeatures();
    this.cdr.detectChanges();
  }
}

