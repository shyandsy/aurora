import { Injectable, TemplateRef, signal } from '@angular/core';

/**
 * 「壳 + 路由出口」型 tab 页(如用户中心 user-permission):tab 条在壳组件里,
 * 子页经 <router-outlet> 进来,没法直接内容投影。用传送:子页用 [tabShellActions]
 * 指令声明一段操作模板,注册到这里;壳组件把它渲染到 tab 行右侧。切子路由时自动换/清。
 */
@Injectable({ providedIn: 'root' })
export class TabShellActionsService {
  readonly template = signal<TemplateRef<unknown> | null>(null);

  register(tpl: TemplateRef<unknown>): void {
    this.template.set(tpl);
  }

  /** 只在当前就是这个模板时才清,避免「新子页先注册、旧子页后销毁」把新的清掉。 */
  unregister(tpl: TemplateRef<unknown>): void {
    if (this.template() === tpl) {
      this.template.set(null);
    }
  }
}
