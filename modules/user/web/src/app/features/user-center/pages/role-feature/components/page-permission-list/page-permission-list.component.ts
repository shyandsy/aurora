import { Component, inject, OnInit, ChangeDetectorRef, DestroyRef, input, output, effect } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { CommonModule } from '@angular/common';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { forkJoin, Observable } from 'rxjs';
import { UserCenterApi } from '../../../../shared/services/user-center-api';
import { Feature } from '../../../../shared/models/feature.dto';
import { RoleFeature } from '../../../../shared/models/role-feature.dto';
import { I18nService } from '../../../../../../core/services/i18n.service';

type Tier = 'none' | 'view' | 'operate';

interface PageRow {
  module: string;
  label: string;
  labelI18n?: Record<string, string>;
  sortOrder: number;
  viewFeatureId: number | null;
  operateFeatureId: number | null;
  tier: Tier;
  updating: boolean;
}

/**
 * 页面级权限授权:每页三档(无 / 只读 view / 可操作 operate)。
 * 授的是 ui.page.<module>.view / .operate 这一档 UI feature;真正的一整簇业务 API feature
 * 由后端「签发时展开」(见 doc/rbac/ui-feature-view-operate.md)。这里只写单条 role_features,不展开。
 * operate 隐含 view:切到 operate 会移除 view、加 operate,反之亦然。
 *
 * 页目录(有哪些页、页名、顺序)完全由数据驱动:从 /feature 拉回的 ui.page.* 行派生,
 * 页名取该行 label_i18n 的当前语言值(由各项目 data 套迁移 seed),顺序取 sort_order。
 * 本组件不写死任何业务页名 —— 这是「机制归 aurora、数据归项目」的前端落点。
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
  private readonly translate = inject(TranslateService);
  private readonly i18n = inject(I18nService);
  private readonly destroyRef = inject(DestroyRef);

  selectedRoleId = input<number | null>(null);
  refreshTrigger = input<number>(0);
  // 授权变化(供宿主刷新「高级」列表)
  changed = output<void>();

  private features: Feature[] = [];
  private roleFeatures: RoleFeature[] = [];
  rows: PageRow[] = [];
  searchTerm = '';
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
    // 语言切换时重算页名(label 取当前语言值)
    this.translate.onLangChange.pipe(takeUntilDestroyed(this.destroyRef)).subscribe(() => {
      this.rebuildRows();
      this.cdr.detectChanges();
    });
  }

  ngOnInit(): void {
    this.loadFeatures();
  }

  /** 拉全量 feature(派生页目录 + 取 ui.page.*.view/operate 的 id)。只需一次。 */
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

  private hasFeature(featureId: number | null): boolean {
    return featureId != null && this.roleFeatures.some(rf => rf.featureId === featureId);
  }

  /** 当前语言('zh-CN' | 'zh-TW' | 'en',与 label_i18n 的 key 一致),用于取页名。 */
  private currentLang(): string {
    return this.i18n.getCurrentLanguage();
  }

  /** 从 label_i18n 取当前语言页名:当前语言 → zh-CN → 任意一个 → module 兜底(永不显示空)。 */
  private pickLabel(labelI18n: Record<string, string> | undefined, module: string): string {
    if (labelI18n) {
      const lang = this.currentLang();
      const v = labelI18n[lang] ?? labelI18n['zh-CN'] ?? Object.values(labelI18n)[0];
      if (v) {
        return v;
      }
    }
    return module;
  }

  /**
   * 从 features 派生页目录:匹配 ui.page.<module>.(view|operate),按 module 归并出每页的
   * view/operate id + 页名 + 顺序。页名/顺序取该 module 任一 ui.page 行的 label_i18n/sort_order。
   */
  private rebuildRows(): void {
    const updatingModules = new Set(this.rows.filter(r => r.updating).map(r => r.module));
    const byModule = new Map<string, {
      viewId: number | null; operateId: number | null; labelI18n?: Record<string, string>; sortOrder: number;
    }>();

    const re = /^ui\.page\.([a-z0-9_]+)\.(view|operate)$/;
    for (const f of this.features) {
      const m = f.name.match(re);
      if (!m) {
        continue;
      }
      const module = m[1];
      const tier = m[2];
      const entry = byModule.get(module) ?? { viewId: null, operateId: null, labelI18n: undefined, sortOrder: 0 };
      if (tier === 'view') {
        entry.viewId = f.id;
      } else {
        entry.operateId = f.id;
      }
      // label_i18n / sort_order 取任一有值的行(view/operate 两行通常一致)
      if (f.labelI18n && Object.keys(f.labelI18n).length > 0 && !entry.labelI18n) {
        entry.labelI18n = f.labelI18n;
      }
      if (f.sortOrder != null && f.sortOrder !== 0 && entry.sortOrder === 0) {
        entry.sortOrder = f.sortOrder;
      }
      byModule.set(module, entry);
    }

    this.rows = Array.from(byModule.entries())
      .map(([module, e]) => {
        let tier: Tier = 'none';
        if (this.hasFeature(e.operateId)) {
          tier = 'operate';
        } else if (this.hasFeature(e.viewId)) {
          tier = 'view';
        }
        return {
          module,
          label: this.pickLabel(e.labelI18n, module),
          labelI18n: e.labelI18n,
          sortOrder: e.sortOrder,
          viewFeatureId: e.viewId,
          operateFeatureId: e.operateId,
          tier,
          updating: updatingModules.has(module),
        } as PageRow;
      })
      .filter(r => r.viewFeatureId != null || r.operateFeatureId != null)
      .sort((a, b) => (a.sortOrder - b.sortOrder) || a.label.localeCompare(b.label));
  }

  /** 搜索过滤后的行(按页名,忽略大小写)。 */
  get filteredRows(): PageRow[] {
    const term = this.searchTerm.trim().toLowerCase();
    if (!term) {
      return this.rows;
    }
    return this.rows.filter(r => r.label.toLowerCase().includes(term));
  }

  get grantedCount(): number {
    return this.rows.filter(r => r.tier !== 'none').length;
  }

  get totalCount(): number {
    return this.rows.length;
  }

  canEdit(): boolean {
    const roleId = this.selectedRoleId();
    return !!roleId && roleId > 0 && !this.isLoading;
  }

  /** 为某行切到目标档,算出要加/要删的 role_feature 调用(不发起)。 */
  private buildCalls(row: PageRow, target: Tier): Observable<unknown>[] {
    const roleId = this.selectedRoleId();
    if (!roleId || roleId <= 0) {
      return [];
    }
    const want = new Set<number>();
    if (target === 'view' && row.viewFeatureId != null) {
      want.add(row.viewFeatureId);
    }
    if (target === 'operate' && row.operateFeatureId != null) {
      want.add(row.operateFeatureId);
    }
    const pageFeatureIds = [row.viewFeatureId, row.operateFeatureId].filter((x): x is number => x != null);
    const calls: Observable<unknown>[] = [];
    for (const fid of want) {
      if (!this.hasFeature(fid)) {
        calls.push(this.apiService.post<RoleFeature>('/role-feature', { roleId, featureId: fid }));
      }
    }
    for (const fid of pageFeatureIds) {
      if (!want.has(fid) && this.hasFeature(fid)) {
        const rf = this.roleFeatures.find(x => x.featureId === fid);
        if (rf) {
          calls.push(this.apiService.delete<{ message: string }>(`/role-feature/${rf.id}`));
        }
      }
    }
    return calls;
  }

  /** 单行切档。 */
  onTierChange(row: PageRow, target: Tier): void {
    if (!this.canEdit() || row.updating || target === row.tier) {
      return;
    }
    // operate 档不可达(无 operate feature)时忽略
    if (target === 'operate' && row.operateFeatureId == null) {
      return;
    }
    const calls = this.buildCalls(row, target);
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

  /** 批量把当前(过滤后)所有页设为目标档。 */
  setAll(target: Tier): void {
    if (!this.canEdit()) {
      return;
    }
    const targets = this.filteredRows.filter(r =>
      !r.updating &&
      r.tier !== target &&
      !(target === 'operate' && r.operateFeatureId == null)
    );
    if (targets.length === 0) {
      return;
    }
    const calls: Observable<unknown>[] = [];
    for (const row of targets) {
      const rowCalls = this.buildCalls(row, target);
      if (rowCalls.length > 0) {
        row.updating = true;
        calls.push(...rowCalls);
      }
    }
    if (calls.length === 0) {
      return;
    }
    this.error = null;
    this.cdr.detectChanges();
    forkJoin(calls).subscribe({
      next: () => {
        this.loadRoleFeatures();
        this.changed.emit();
      },
      error: (err) => {
        console.error('Error bulk updating page permissions:', err);
        this.error = 'Failed to update page permissions';
        this.loadRoleFeatures();
        this.cdr.detectChanges();
      }
    });
  }
}
