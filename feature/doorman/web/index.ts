/**
 * doorman 特性模块对外出口(barrel)。
 *
 * Native Federation 暴露的是组件 `DoormanConfigComponent`(federation.config.js `./Config` 指向
 * doorman-config.component.ts)。宿主用
 *   `loadRemoteModule('doorman','./Config').then(m => m.DoormanConfigComponent)`
 * 拿到组件并绑定 [scope] / [apiBase]。
 */
export { DoormanConfigComponent } from './doorman-config.component';
export type {
  KindsResponse,
  DoormanKind,
  DoormanFieldSchema,
  DoormanRule,
  SaveDoormanRuleRequest,
} from './shared/models/doorman.dto';
