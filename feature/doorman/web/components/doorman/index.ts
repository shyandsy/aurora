/**
 * doorman 特性模块对外出口(barrel)。
 *
 * Native Federation 暴露两个组件:
 *   - `./Config`     → DoormanConfigComponent(规则配置面板;绑定 [scope]/[scopes] + [apiBase])
 *   - `./Decisions`  → DoormanDecisionsModalComponent(决策明细弹窗;绑定 [apiBase] + [scope],监听 (close))
 * 宿主用 `loadRemoteModule('doorman','./Config'|'./Decisions').then(m => m.XxxComponent)` 拿组件。
 */
export { DoormanConfigComponent } from './doorman-config.component';
export { DoormanDecisionsModalComponent } from './decisions-modal/doorman-decisions-modal.component';
export type {
  KindsResponse,
  DoormanKind,
  DoormanFieldSchema,
  DoormanRule,
  SaveDoormanRuleRequest,
  StatsResponse,
  DecisionRow,
  DecisionsPageResponse,
} from './shared/models/doorman.dto';
