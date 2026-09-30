import { Component, input, output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TranslateModule } from '@ngx-translate/core';

/** 单个 tab 的描述。id 用作选中判断与 change 事件载荷。 */
export interface TabItem {
  /** 唯一标识,选中态比对与 tabChange 事件都用它。 */
  id: string;
  /** i18n key(优先):走 translate 管道。 */
  labelKey?: string;
  /** 原始文案(labelKey 缺省时用)。 */
  label?: string;
  /** 可选的数字角标(如条数),为 null/undefined 时不显示。 */
  count?: number | null;
  /** 置灰且不可点。 */
  disabled?: boolean;
}

/**
 * 通用下划线风格 tab 条(Material / Ant Design 风):底部通栏细线,选中项 indigo 下划线 + 文字高亮。
 * 受控组件 —— activeId 由父组件持有(signal/属性均可),点击只发 tabChange,由父组件回写。
 *
 * 用法:
 *   <app-tab-bar [tabs]="tabs" [activeId]="activeTab()" (tabChange)="selectTab($any($event))" />
 */
@Component({
  selector: 'app-tab-bar',
  standalone: true,
  imports: [CommonModule, TranslateModule],
  templateUrl: './tab-bar.component.html',
  styleUrl: './tab-bar.component.css'
})
export class TabBarComponent {
  /** tab 列表。 */
  tabs = input.required<TabItem[]>();
  /** 当前选中的 tab id。 */
  activeId = input.required<string>();

  /** 选中变化,载荷为被点击 tab 的 id。 */
  tabChange = output<string>();

  onSelect(tab: TabItem): void {
    if (tab.disabled || tab.id === this.activeId()) {
      return;
    }
    this.tabChange.emit(tab.id);
  }
}
