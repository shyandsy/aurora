/**
 * 看门人(doorman)规则配置的数据契约 —— **风险评估器模型**。
 *
 * 一条规则 = 多个条件(AND/OR 组合)命中后给出**一个风险等级**;门房只判定风险,不执行动作。
 * 「风险等级 → 具体动作」由业务侧(如注册流程)决定,不在门房配置。
 *
 * 严格对齐后端 doorman 管理 API(base 由宿主传入,如 `/api/admin/v1/doorman`):
 *   - GET  {apiBase}/kinds          → KindsResponse(条件类别 + 各自字段 schema + 风险等级枚举)
 *   - GET  {apiBase}/rules?scope=x  → DoormanRule[]
 *   - POST {apiBase}/rules          → DoormanRule(id 省略/0 = 新增)
 *   - PUT  {apiBase}/rules/{id}     → DoormanRule
 *   - DELETE {apiBase}/rules/{id}   → { ok: true }
 *
 * 关键:条件的**类型与字段全部来自 kinds 接口(schema)**,前端不写死任何条件类型或字段;
 * 后端加插件(新条件类型/新字段)时前端零改动,靠这些 schema 动态渲染。
 */

/** 字段控件类型(后端 schema 的 field.type)。未知类型前端兜底成文本框。 */
export type DoormanFieldType =
  | 'string'
  | 'string_list'
  | 'int'
  | 'int_list'
  | 'duration'
  | 'select';

/** 条件组合方式:全部满足(and)/ 任一满足(or)。 */
export type DoormanCombine = 'and' | 'or';

/** 风险等级枚举(由低到高)。来自 GET /kinds 的 riskLevels。 */
export type DoormanRiskLevel = 'none' | 'low' | 'medium' | 'high' | 'critical';

/** select 类型字段的一个候选项。后端可给字符串,或 { value, label/labelKey } 对象,两者都支持。 */
export type DoormanFieldOption =
  | string
  | {
      value: string;
      /** i18n key(优先);无则用 label;再无则显示 value。 */
      labelKey?: string;
      label?: string;
    };

/** 一个条件类别里的单个配置字段(schema)。 */
export interface DoormanFieldSchema {
  /** 参数键名(写进 condition.params 的 key)。 */
  key: string;
  /** 控件类型。 */
  type: DoormanFieldType;
  /** i18n key;有对应文案就用,没有就直接显示该 key(或 field.key)。 */
  labelKey?: string;
  /** 是否必填(前端做轻校验;权威校验在后端)。 */
  required?: boolean;
  /** select 类型的候选项。 */
  options?: DoormanFieldOption[];
  /** 可选的占位提示(如 duration 示例 "1h")。 */
  placeholder?: string;
}

/** 一个条件类别(schema)。对应后端 KindInfo。 */
export interface DoormanKind {
  /** 类别标识(如 "ua_match" / "asn_hosting" / "country_in")。 */
  type: string;
  /** 可选的展示名 i18n key(后端给了就用,没有就走约定 doorman.kind.<type> → 原始 type)。 */
  labelKey?: string;
  /** 该类别的配置字段;空数组表示无参数(如 "asn_hosting")。 */
  fields: DoormanFieldSchema[];
}

/** GET {apiBase}/kinds 的响应(新形状:conditions + riskLevels,不再有 actions)。 */
export interface KindsResponse {
  conditions: DoormanKind[];
  /** 风险等级枚举,由低到高,如 ["none","low","medium","high","critical"]。 */
  riskLevels: DoormanRiskLevel[];
}

/** 参数对象:任意 key → 值(string / number / string[] / number[])。是 JSON 对象,不是字符串。 */
export type DoormanParams = Record<string, unknown>;

/** 规则里的单个条件。asn_hosting 这类无参数条件 params 省略/空对象。 */
export interface ConditionDTO {
  type: string;
  params?: DoormanParams;
}

/** 一条规则(GET/POST/PUT 返回体) —— 多条件 + 组合方式 + 风险等级。 */
export interface DoormanRule {
  id: number;
  scope: string;
  name: string;
  conditions: ConditionDTO[];
  combine: DoormanCombine;
  riskLevel: DoormanRiskLevel;
  enabled: boolean;
  created?: string;
  modified?: string;
}

/** 新增/编辑规则的请求体(id 省略/0 = 新增)。 */
export interface SaveDoormanRuleRequest {
  scope: string;
  name: string;
  conditions: ConditionDTO[];
  combine: DoormanCombine;
  riskLevel: DoormanRiskLevel;
  enabled: boolean;
}

/** DELETE {apiBase}/rules/{id} 的响应。 */
export interface DeleteDoormanRuleResponse {
  ok: boolean;
}

/** GET {apiBase}/policy/kinds?scope= 的响应:配置「风险→动作」所需的选项。 */
export interface PolicyKindsResponse {
  /** 风险等级枚举(由低到高)。 */
  riskLevels: DoormanRiskLevel[];
  /** 该 scope 可选的动作名清单(下拉候选;由业务在后端登记)。空 = 该 scope 没登记动作。 */
  actions: string[];
}

/** 「风险等级 → 动作名」映射。key 是风险等级,value 是动作名;某等级缺省 = 走业务内置默认。 */
export type ActionPolicy = Record<string, string>;

/** 「判定→结果」漏斗:分母=判定后有后续的决策(如判要激活),resolved=已完成。未完成=challenged−resolved。 */
export interface FunnelResponse {
  challenged: number; // 需后续(如判要激活)
  resolved: number; // 已完成(如激活成功)
}

/** GET {apiBase}/stats?scope=&days= 的响应:门禁决策统计(近 N 天)。 */
export interface StatsResponse {
  sinceDays: number;
  total: number;
  /** 风险等级 → 命中数。 */
  byRisk: Record<string, number>;
  /** 动作名 → 数。 */
  byAction: Record<string, number>;
  /** 判定→结果漏斗(判要激活→激活/放弃)。 */
  funnel: FunnelResponse;
}

/** 一条决策明细(GET {apiBase}/decisions 返回的数组元素)。 */
export interface DecisionRow {
  id: number;
  riskLevel: string;
  action: string;
  matched: string; // 命中规则名,逗号分隔
  subject?: string; // 关联键/标识(注册场景=邮箱);完成回填后被清空
  ua?: string;
  ip?: string;
  country?: string;
  isp?: string; // 运营商(仅国内)
  asn?: number;
  asnOrg?: string;
  isHosting: boolean;
  outcome?: string; // 后续结果(如 "activated");空 = 无后续 / 判了要激活但未完成
  created: string;
}
