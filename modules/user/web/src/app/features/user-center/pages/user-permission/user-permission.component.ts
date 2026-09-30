import { Component, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterOutlet, Router, ActivatedRoute } from '@angular/router';
import { TabBarComponent, TabItem } from '../../../../shared/components/tab-bar/tab-bar.component';
import { AuthService } from '../../../../core/services/auth.service';

/** 壳内 tab:可选 feature 门控(缺省=登录即可见)。 */
interface GatedTab extends TabItem {
  /** 需要的 UI 门控 feature;缺省表示无门控。 */
  feature?: string;
}

/**
 * 用户中心壳组件:把「账号 / 角色权限 / Microservice」三个页面收进一个入口,顶部 tab 切换。
 * 账号、角色权限两 tab 不带 feature,登录即可见;Microservice tab 按 `ui.menu.microservice`
 * 门控(与 home 落地页入口、独立/federated 路由上的 featureGuard 同一 feature),没权限不显示。
 * tab 样式复用共享 app-tab-bar(与 system-setting 一致);底层仍是路由:activeId 从当前 URL 推,
 * 点击 tab 走 router.navigate。
 *
 * 关键(Native Federation):tab 切换用**相对导航**(relativeTo 本路由),不写死绝对 `/user-permission`。
 * 独立运行时本路由挂在根 → 解析成 `/user-permission/<id>`(与原绝对写法等价);被 admin 宿主 federated
 * 挂到 `/user-center` 下时 → 解析成 `/user-center/user-permission/<id>`。写死绝对路径会在宿主里指向
 * 一条不存在的路由 → tab 点击 404,故必须相对。
 */
@Component({
  selector: 'app-user-permission',
  standalone: true,
  imports: [CommonModule, RouterOutlet, TabBarComponent],
  templateUrl: './user-permission.component.html'
})
export class UserPermissionComponent {
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);
  private readonly authService = inject(AuthService);

  /** 全部 tab(含门控声明);实际渲染的以 visibleTabs() 为准。 */
  private readonly allTabs: GatedTab[] = [
    { id: 'users', labelKey: 'sidebar.users' },
    { id: 'roles', labelKey: 'sidebar.roles' },
    { id: 'microservice', labelKey: 'sidebar.microservice', feature: 'ui.menu.microservice' },
    // 第 4 个 tab:被锁登录(loginguard 后台)。门控用后端实际校验的 user.get(查看语义,列表接口即此)——
    // 后端刻意复用用户权限、未新增 ui.menu.*,故不另造门控 key(否则要等后端 seed 才可见)。
    { id: 'locked-logins', labelKey: 'sidebar.lockedLogins', feature: 'user.get' }
  ];

  /** 当前账号可见的 tab:无 feature 的恒显,有 feature 的走 authService.hasFeature(含 '*' 超管通配)。 */
  visibleTabs(): TabItem[] {
    return this.allTabs.filter(t => !t.feature || this.authService.hasFeature(t.feature));
  }

  activeTabId(): string {
    const tabs = this.visibleTabs();
    const segs = this.router.url.split('?')[0].split('/').filter(Boolean);
    const last = segs[segs.length - 1];
    return tabs.some(t => t.id === last) ? last : tabs[0].id;
  }

  selectTab(id: string): void {
    // 相对本路由(user-permission)导航到子 tab,独立/federated 两种挂载点都正确(见类注释)。
    this.router.navigate([id], { relativeTo: this.route });
  }
}
