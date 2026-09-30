import { Component, inject, OnInit, ChangeDetectorRef, input, output, effect } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TranslateModule } from '@ngx-translate/core';
import { forkJoin, Observable } from 'rxjs';
import { UserCenterApi } from '../../../../shared/services/user-center-api';
import { Feature } from '../../../../shared/models/feature.dto';
import { RoleFeature } from '../../../../shared/models/role-feature.dto';

type Tier = 'none' | 'view' | 'operate';

interface PageRow {
  module: string;
  labelKey: string;
  viewFeatureId: number | null;
  operateFeatureId: number | null;
  tier: Tier;
  updating: boolean;
}

// 页面展示顺序 + 复用 sidebar.* 标签(与侧边栏一致,管理员一眼能对应)。module == 迁移里的 ui.page.<module>.*。
const PAGE_LABELS: Array<{ module: string; labelKey: string }> = [
  { module: 'customer', labelKey: 'sidebar.customers' },
  { module: 'analytics', labelKey: 'sidebar.analytics' },
  { module: 'tags', labelKey: 'sidebar.tags' },
  { module: 'customer_tier', labelKey: 'sidebar.customerTier' },
  { module: 'product', labelKey: 'sidebar.products' },
  { module: 'order', labelKey: 'sidebar.orders' },
  { module: 'payment_account', labelKey: 'sidebar.paymentAccount' },
  { module: 'announcement', labelKey: 'sidebar.announcement' },
  { module: 'traffic', labelKey: 'sidebar.traffic' },
  { module: 'commission', labelKey: 'sidebar.commission' },
  { module: 'withdrawal', labelKey: 'sidebar.withdrawal' },
  { module: 'release', labelKey: 'sidebar.release' },
  { module: 'app_config', labelKey: 'sidebar.appConfig' },
  { module: 'server', labelKey: 'sidebar.server' },
  { module: 'storage_source', labelKey: 'sidebar.storageSource' },
  { module: 'microservice', labelKey: 'sidebar.microservice' },
  { module: 'xray_audit', labelKey: 'sidebar.xrayAudit' },
  { module: 'tasks', labelKey: 'sidebar.taskCenter' },
  { module: 'ticket', labelKey: 'sidebar.tickets' },
  { module: 'document', labelKey: 'sidebar.documents' },
  { module: 'document_category', labelKey: 'sidebar.documentCategories' },
  { module: 'system_setting', labelKey: 'sidebar.systemSetting' },
  { module: 'user_permission', labelKey: 'sidebar.userPermission' },
];

/**
 * 页面级权限授权:每页三档(无 / 只读 view / 可操作 operate)。
 * 授的是 ui.page.<module>.view / .operate 这一档 UI feature;真正的一整簇业务 API feature
 * 由后端「签发时展开」(见 doc/rbac/ui-feature-view-operate.md)。这里只写单条 role_features,不展开。
 * operate 隐含 view:切到 operate 会移除 view、加 operate,反之亦然。
 */
@Component({
  selector: 'app-page-permission-list',
  standalone: true,
  imports: [CommonModule, TranslateModule],
  templateUrl: './page-permission-list.component.html',
  styleUrl: './page-permission-list.component.css'
})
export class PagePermissionListComponent implements OnInit {
  private readonly apiService = inject(UserCenterApi);
  private readonly cdr = inject(ChangeDetectorRef);

  selectedRoleId = input<number | null>(null);
  refreshTrigger = input<number>(0);
  // 授权变化(供宿主刷新「高级」列表)
  changed = output<void>();

  private features: Feature[] = [];
  private roleFeatures: RoleFeature[] = [];
  rows: PageRow[] = [];
  isLoading = false;
  error: string | null = null;

  constructor() {
    effect(() => {
      // 依赖 refreshTrigger / selectedRoleId 变化时重载角色授权
      this.refreshTrigger();
      const roleId = this.selectedRoleId();
      if (roleId && roleId > 0) {
        this.loadRoleFeatures();
      } else {
        this.roleFeatures = [];
        this.rebuildRows();
      }
    });
  }

  ngOnInit(): void {
    this.loadFeatures();
  }

  /** 拉全量 feature(取 ui.page.*.view/operate 的 id)。只需一次。 */
  private loadFeatures(): void {
    this.isLoading = true;
    this.error = null;
    this.cdr.detectChanges();
    this.apiService.get<Feature[]>('/feature').subscribe({
      next: (data) => {
        this.features = Array.isArray(data) ? data : [];
        this.isLoading = false;
        if (this.selectedRoleId()) {
          this.loadRoleFeatures();
        } else {
          this.rebuildRows();
        }
        this.cdr.detectChanges();
      },
      error: (err) => {
        console.error('Error loading features:', err);
        this.error = 'Failed to load features';
        this.isLoading = false;
        this.cdr.detectChanges();
      }
    });
  }

  private loadRoleFeatures(): void {
    const roleId = this.selectedRoleId();
    if (!roleId || roleId <= 0) {
      this.roleFeatures = [];
      this.rebuildRows();
      return;
    }
    this.apiService.get<RoleFeature[]>(`/role-feature?roleId=${roleId}`).subscribe({
      next: (data) => {
        this.roleFeatures = Array.isArray(data) ? data : [];
        this.rebuildRows();
        this.cdr.detectChanges();
      },
      error: (err) => {
        console.error('Error loading role features:', err);
        this.roleFeatures = [];
        this.rebuildRows();
        this.cdr.detectChanges();
      }
    });
  }

  private featureIdByName(name: string): number | null {
    const f = this.features.find(x => x.name === name);
    return f ? f.id : null;
  }

  private hasFeature(featureId: number | null): boolean {
    return featureId != null && this.roleFeatures.some(rf => rf.featureId === featureId);
  }

  /** 依据 features + roleFeatures 重算每页当前档位。保留正在更新的行标记。 */
  private rebuildRows(): void {
    const updatingModules = new Set(this.rows.filter(r => r.updating).map(r => r.module));
    this.rows = PAGE_LABELS.map(({ module, labelKey }) => {
      const viewId = this.featureIdByName(`ui.page.${module}.view`);
      const operateId = this.featureIdByName(`ui.page.${module}.operate`);
      let tier: Tier = 'none';
      if (this.hasFeature(operateId)) {
        tier = 'operate';
      } else if (this.hasFeature(viewId)) {
        tier = 'view';
      }
      return {
        module,
        labelKey,
        viewFeatureId: viewId,
        operateFeatureId: operateId,
        tier,
        updating: updatingModules.has(module),
      };
    }).filter(r => r.viewFeatureId != null || r.operateFeatureId != null);
  }

  canEdit(): boolean {
    const roleId = this.selectedRoleId();
    return !!roleId && roleId > 0 && !this.isLoading;
  }

  /** <select> 切档:算出要加/要删的 role_feature,forkJoin 执行后重载。 */
  onTierChange(row: PageRow, target: Tier): void {
    const roleId = this.selectedRoleId();
    if (!roleId || roleId <= 0 || row.updating) {
      return;
    }
    if (target === row.tier) {
      return;
    }

    // 目标档需要持有的 featureId 集合
    const want = new Set<number>();
    if (target === 'view' && row.viewFeatureId != null) {
      want.add(row.viewFeatureId);
    }
    if (target === 'operate' && row.operateFeatureId != null) {
      want.add(row.operateFeatureId);
    }

    // 本页涉及的两个 featureId,当前哪些已授
    const pageFeatureIds = [row.viewFeatureId, row.operateFeatureId].filter((x): x is number => x != null);
    const calls: Observable<unknown>[] = [];

    // 需要新增:want 里当前未授的
    for (const fid of want) {
      if (!this.hasFeature(fid)) {
        calls.push(this.apiService.post<RoleFeature>('/role-feature', { roleId, featureId: fid }));
      }
    }
    // 需要移除:本页已授、但不在 want 里的
    for (const fid of pageFeatureIds) {
      if (!want.has(fid) && this.hasFeature(fid)) {
        const rf = this.roleFeatures.find(x => x.featureId === fid);
        if (rf) {
          calls.push(this.apiService.delete<{ message: string }>(`/role-feature/${rf.id}`));
        }
      }
    }

    if (calls.length === 0) {
      row.tier = target;
      return;
    }

    row.updating = true;
    this.error = null;
    this.cdr.detectChanges();
    forkJoin(calls).subscribe({
      next: () => {
        row.updating = false;
        this.loadRoleFeatures();
        this.changed.emit();
      },
      error: (err) => {
        console.error('Error updating page permission:', err);
        this.error = 'Failed to update page permission';
        row.updating = false;
        this.loadRoleFeatures();
        this.cdr.detectChanges();
      }
    });
  }
}
