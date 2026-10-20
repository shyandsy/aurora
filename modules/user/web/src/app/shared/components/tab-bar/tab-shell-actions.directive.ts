import { Directive, TemplateRef, inject, OnInit, OnDestroy } from '@angular/core';
import { TabShellActionsService } from './tab-shell-actions.service';

/**
 * 壳 + 路由出口 tab 页的子页,用它把操作按钮传送到壳的 tab 行右侧:
 *
 *   <ng-template tabShellActions>
 *     <button (click)="onCreate()">创建用户</button>
 *   </ng-template>
 *
 * 壳组件读 TabShellActionsService.template 用 ngTemplateOutlet 渲染。子页 destroy 自动注销。
 */
@Directive({ selector: '[tabShellActions]', standalone: true })
export class TabShellActionsDirective implements OnInit, OnDestroy {
  private readonly tpl = inject(TemplateRef);
  private readonly svc = inject(TabShellActionsService);

  ngOnInit(): void {
    this.svc.register(this.tpl);
  }

  ngOnDestroy(): void {
    this.svc.unregister(this.tpl);
  }
}
