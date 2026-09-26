import { Component, inject, input, effect, signal, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { HttpErrorResponse } from '@angular/common/http';
import { TimeoutError } from 'rxjs';
import { DoormanApi } from './shared/services/doorman-api';
import {
  KindsResponse,
  DoormanKind,
  DoormanFieldSchema,
  DoormanFieldOption,
  DoormanRule,
  DoormanParams,
  ConditionDTO,
  DoormanCombine,
  DoormanRiskLevel,
  SaveDoormanRuleRequest,
  PolicyKindsResponse,
  ActionPolicy,
  StatsResponse,
  DecisionRow,
} from './shared/models/doorman.dto';
import { DOORMAN_I18N } from './i18n';
import { ConfirmDialogComponent } from './shared/components/confirm-dialog/confirm-dialog.component';
import { formatDate } from './shared/utils/date.util';

/** 编辑器里的一条可编辑条件(type + params;params 随所选类型的 schema 动态渲染)。 */
interface EditCondition {
  type: string;
  params: DoormanParams;
}

/** 风险等级枚举兜底(后端 /kinds 会返回;拿不到时用它保证选择器可用)。由低到高。 */
const RISK_LEVELS: DoormanRiskLevel[] = ['none', 'low', 'medium', 'high', 'critical'];

/**
 * 看门人(doorman)规则配置组件 —— Native Federation 暴露的 `./Config`。
 *
 * **风险评估器模型**:一条规则 = 多个条件(AND/OR 组合)命中后给出一个风险等级;门房只判定,不执行动作。
 *
 * 宿主用 `loadRemoteModule('doorman','./Config').then(m => m.DoormanConfigComponent)` 拿到本组件,
 * 挂进自己界面并绑定 `[scope]`(如 "register")和 `[apiBase]`(如 `/api/admin/v1/doorman`)。
 *
 * schema 驱动:加载时先 `GET {apiBase}/kinds` 拿到所有条件类别及各自字段 schema + 风险等级枚举,据此
 * **动态渲染**条件参数表单;条件类型和字段一律不写死在前端,后端加插件前端零改动。
 */
@Component({
  selector: 'app-doorman-config',
  standalone: true,
  imports: [CommonModule, FormsModule, TranslateModule, ConfirmDialogComponent],
  providers: [DoormanApi],
  templateUrl: './doorman-config.component.html',
  styleUrl: './doorman-config.component.css',
})
export class DoormanConfigComponent {
  private readonly api = inject(DoormanApi);
  private readonly translate = inject(TranslateService, { optional: true });

  // ---- 宿主输入 ----------------------------------------------------------
  /** 单作用域(向后兼容)。宿主只配一个场景时用它;多场景用 scopes。 */
  scope = input<string>('');
  /** 作用域列表(顶部 tab 切换)。宿主决定有哪些场景(如 register/login/reset_password),标签走 doorman.scope.<id> i18n。 */
  scopes = input<string[]>([]);
  /** 后端 doorman 管理 API 的基路径,如 `/api/admin/v1/doorman`。 */
  apiBase = input<string>('');

  // ---- 作用域 tab ---------------------------------------------------------
  /** 当前选中的作用域(tab)。 */
  activeScope = signal<string>('');
  /** 要展示的作用域列表:优先 scopes 输入,否则回退到单个 scope 输入。 */
  readonly scopeList = computed<string[]>(() => {
    const ss = this.scopes();
    if (Array.isArray(ss) && ss.length > 0) {
      return ss;
    }
    const s = this.scope();
    return s ? [s] : [];
  });

  /** 切换作用域 tab。 */
  selectScope(s: string): void {
    if (s !== this.activeScope()) {
      this.view.set('rules');  // 切场景回规则视图(新场景可能未接入,停在空明细页没意义)
      this.activeScope.set(s); // 触发 effect 重载该 scope 的规则 + 策略 + 统计
    }
  }

  /** 作用域展示名:doorman.scope.<id> 有就用,否则显示原始 id。 */
  scopeLabel(s: string): string {
    return this.labelFor(`doorman.scope.${s}`, s);
  }

  // ---- 数据态(signal:zoneless 下自动触发变更检测) -----------------------
  kinds = signal<KindsResponse | null>(null);
  rules = signal<DoormanRule[]>([]);
  loading = signal(false);
  listError = signal<string | null>(null);

  // ---- 编辑器态 ----------------------------------------------------------
  showEditor = signal(false);
  editingId = signal<number | null>(null);
  editorError = signal<string | null>(null);
  submitting = signal(false);

  // 编辑器表单模型(plain 字段 + ngModel;DOM 事件在 zoneless 下会调度变更检测)。
  editName = '';
  editEnabled = true;
  editCombine: DoormanCombine = 'and';
  editRiskLevel: DoormanRiskLevel = 'none';
  editConditions: EditCondition[] = [];

  // ---- 删除确认 ----------------------------------------------------------
  ruleToDelete = signal<DoormanRule | null>(null);

  // ---- 「风险等级 → 动作」策略态 ----------------------------------------
  policyKinds = signal<PolicyKindsResponse | null>(null);
  /** 可编辑的映射 {风险等级: 动作名};空串 = 用业务内置默认。 */
  policy = signal<ActionPolicy>({});
  policySaving = signal(false);
  policyError = signal<string | null>(null);
  policySaved = signal(false);
  /** 策略弹窗开关:策略是该场景的一份独立配置,点工具栏按钮才打开、弹窗里编辑保存。 */
  showPolicyEditor = signal(false);
  /** 打开弹窗时的快照;取消时回滚,避免未保存的改动残留在页面态。 */
  private policyBackup: ActionPolicy = {};

  // ---- 决策统计 + 明细态 -------------------------------------------------
  stats = signal<StatsResponse | null>(null);
  decisions = signal<DecisionRow[]>([]);
  /** 视图切换:规则配置 ↔ 决策明细(同一联邦组件内两个视图,明细入口是配置页工具栏上的按钮)。 */
  view = signal<'rules' | 'decisions'>('rules');
  /** 明细翻页态:加载中 / 是否还有下一页(游标 = 当前最后一条 id)。 */
  decisionsLoading = signal(false);
  decisionsHasMore = signal(false);
  private readonly decisionsPage = 20;

  readonly conditions = computed<DoormanKind[]>(() => this.kinds()?.conditions ?? []);
  readonly riskLevels = computed<DoormanRiskLevel[]>(() => {
    const lv = this.kinds()?.riskLevels;
    return Array.isArray(lv) && lv.length > 0 ? lv : RISK_LEVELS;
  });

  constructor() {
    this.mergeI18n();

    // apiBase / 作用域就绪或变化即(重新)加载。apiBase 写进 api.baseUrl(HTTP 层据此拼 URL)。
    // activeScope 未定 / 不在列表 → 纠正到列表首个(set 触发本 effect 重跑);合法后才 reload。
    effect(() => {
      const base = this.apiBase();
      const list = this.scopeList();
      const active = this.activeScope();
      this.api.baseUrl = base;
      if (!base || list.length === 0) {
        return;
      }
      if (!list.includes(active)) {
        this.activeScope.set(list[0]);
        return;
      }
      this.reload();
    });
  }

  // ---- i18n:把模块三语字典深合并进(独立=根 / federated=宿主共享单例)ngx-translate ----
  private mergeI18n(): void {
    if (!this.translate) {
      return;
    }
    const apply = (lang: string): void => {
      const dict = DOORMAN_I18N[lang];
      if (dict) {
        this.translate!.setTranslation(lang, dict, true); // shouldMerge=true 深合并,不清宿主已有 key
      }
    };
    Object.keys(DOORMAN_I18N).forEach(apply);
    // 每次语言(重新)加载/切换后再追加一次,防止被宿主主字典 http-loader 整体替换覆盖。
    this.translate.onLangChange.subscribe((e) => apply(e.lang));
  }

  // ---- 加载 --------------------------------------------------------------
  reload(): void {
    this.loading.set(true);
    this.listError.set(null);

    this.api.getKinds().subscribe({
      next: (k) => {
        this.kinds.set(k);
        this.loadRules();
      },
      error: (err) => {
        this.loading.set(false);
        this.listError.set(this.errMsg(err, 'doorman.common.loadFailed'));
      },
    });

    // 「风险→动作」策略与规则各自独立加载(策略失败不连累规则列表)。
    this.loadPolicy();
  }

  private loadPolicy(): void {
    const scope = this.activeScope();
    this.policyError.set(null);
    // ⚠️ 切场景先清空上一个场景的动作目录 + 统计:否则新场景(未接入 / 请求失败)时会残留旧值,
    // hasPolicyActions() 仍为 true → 决策统计/明细入口会错误地漏在「未接入」的 tab(如登录)下。
    this.policyKinds.set(null);
    this.stats.set(null);
    this.api.getPolicyKinds(scope).subscribe({
      next: (pk) => this.policyKinds.set(pk),
      error: () => this.policyKinds.set(null), // 拿不到=按未接入处理,不显示策略/统计
    });
    this.api.getPolicy(scope).subscribe({
      next: (p) => this.policy.set({ ...(p || {}) }),
      error: () => this.policy.set({}),
    });
    // 决策统计(近 7 天):配置页 + 明细页顶部各展示一份,故任何视图下都拉。best-effort(已在上方清空)。
    this.api.getStats(scope, 7).subscribe({
      next: (s) => this.stats.set(s),
      error: () => this.stats.set(null),
    });
    // ⚠️ 这里**绝不能读 this.view()**:loadPolicy 由构造函数的 effect 同步调用,一旦读了 view() 就把 view
    // 变成 effect 的响应依赖 —— 点「决策明细」切 view 会触发整页 reload,来回闪。
    // 明细完全独立于 effect:只由用户操作驱动(openDecisions 进入 / loadMoreDecisions 翻页 / selectScope 切场景)。
  }

  // ---- 统计读数 ----------------------------------------------------------
  riskCount(level: DoormanRiskLevel): number {
    return this.stats()?.byRisk?.[level] ?? 0;
  }
  actionCount(action: string): number {
    return this.stats()?.byAction?.[action] ?? 0;
  }

  // ---- 明细读数 ----------------------------------------------------------
  /** 决策明细的 UA/IP/ASN 摘要(一列展示)。 */
  decisionNet(d: DecisionRow): string {
    const parts: string[] = [];
    if (d.ua) parts.push(d.ua);
    if (d.ip) parts.push(d.ip);
    if (d.country) parts.push(d.country);
    if (d.isp) parts.push(d.isp);
    if (d.asn) parts.push('AS' + d.asn + (d.asnOrg ? ' ' + d.asnOrg : ''));
    return parts.join(' · ') || '—';
  }
  /** 风险等级(字符串)→ pill class,复用 riskPillClass(它按等级取色)。 */
  decisionTime(iso: string): string {
    return formatDate(iso);
  }

  // ---- 视图切换 + 明细翻页 ----------------------------------------------
  /** 进入决策明细视图:切视图并拉第一页。 */
  openDecisions(): void {
    this.view.set('decisions');
    this.loadDecisions(true);
  }

  /** 返回规则配置视图。 */
  backToRules(): void {
    this.view.set('rules');
  }

  /**
   * 拉明细。reset=true:从头(游标 before=0、清空列表);否则以当前最后一条 id 作游标追加下一页。
   * 一页返回条数 = decisionsPage → 认为还有下一页(hasMore),不足 → 到底。
   */
  loadDecisions(reset: boolean): void {
    if (this.decisionsLoading()) {
      return;
    }
    const scope = this.activeScope();
    if (!scope) {
      return;
    }
    const before = reset ? 0 : (this.decisions().at(-1)?.id ?? 0);
    if (reset) {
      this.decisions.set([]);
      this.decisionsHasMore.set(false);
    }
    this.decisionsLoading.set(true);
    this.api.getDecisions(scope, this.decisionsPage, before).subscribe({
      next: (rows) => {
        const list = Array.isArray(rows) ? rows : [];
        this.decisions.set(reset ? list : [...this.decisions(), ...list]);
        this.decisionsHasMore.set(list.length >= this.decisionsPage);
        this.decisionsLoading.set(false);
      },
      error: () => {
        this.decisionsLoading.set(false);
        if (reset) {
          this.decisions.set([]);
        }
      },
    });
  }

  /** 加载下一页(接在末尾)。 */
  loadMoreDecisions(): void {
    this.loadDecisions(false);
  }

  // ---- 「风险等级 → 动作」策略编辑 --------------------------------------
  /** 该 scope 是否登记了动作(没登记就不显示策略区)。 */
  hasPolicyActions(): boolean {
    return (this.policyKinds()?.actions?.length ?? 0) > 0;
  }

  policyActionFor(level: DoormanRiskLevel): string {
    return this.policy()[level] ?? '';
  }

  setPolicyAction(level: DoormanRiskLevel, action: string): void {
    this.policy.set({ ...this.policy(), [level]: action });
    this.policySaved.set(false);
  }

  /** 打开策略弹窗:快照当前映射(供取消回滚),清掉上次的错误/保存提示。 */
  openPolicyEditor(): void {
    this.policyBackup = { ...this.policy() };
    this.policyError.set(null);
    this.policySaved.set(false);
    this.showPolicyEditor.set(true);
  }

  /** 取消:回滚到打开弹窗时的快照(丢弃未保存改动),关闭弹窗。保存中不允许关。 */
  closePolicyEditor(): void {
    if (this.policySaving()) {
      return;
    }
    this.policy.set({ ...this.policyBackup });
    this.policyError.set(null);
    this.showPolicyEditor.set(false);
  }

  /** 动作名 → 展示文案:优先 i18n doorman.action.<type>,回退原始动作名。 */
  actionLabel(action: string): string {
    if (!action) {
      return this.labelFor('doorman.policy.useDefault', '(默认)');
    }
    return this.labelFor(`doorman.action.${action}`, action);
  }

  savePolicy(): void {
    if (this.policySaving()) {
      return;
    }
    // 空串动作(=用默认)不下发,交后端剔除。
    const mapping: ActionPolicy = {};
    for (const [level, action] of Object.entries(this.policy())) {
      if (action) {
        mapping[level] = action;
      }
    }
    this.policySaving.set(true);
    this.policyError.set(null);
    this.policySaved.set(false);
    this.api.setPolicy(this.activeScope(), mapping).subscribe({
      next: () => {
        this.policySaving.set(false);
        this.policySaved.set(true);
        this.policyBackup = { ...this.policy() }; // 已落库,刷新快照
        this.showPolicyEditor.set(false);         // 保存成功即关闭弹窗
      },
      error: (err) => {
        this.policySaving.set(false);
        this.policyError.set(this.errMsg(err, 'doorman.errors.saveFailed'));
      },
    });
  }

  private loadRules(): void {
    this.api.getRules(this.activeScope()).subscribe({
      next: (rules) => {
        this.rules.set(Array.isArray(rules) ? rules : []);
        this.loading.set(false);
      },
      error: (err) => {
        this.loading.set(false);
        this.listError.set(this.errMsg(err, 'doorman.common.loadFailed'));
      },
    });
  }

  // ---- 编辑器 open/close ------------------------------------------------
  openCreate(): void {
    this.editingId.set(null);
    this.editName = '';
    this.editEnabled = true;
    this.editCombine = 'and';
    this.editRiskLevel = this.riskLevels()[0] ?? 'none';
    // 起手给一条空条件(用第一个可用条件类别),避免空白;用户可增删。
    this.editConditions = [this.newCondition()];
    this.editorError.set(null);
    this.showEditor.set(true);
  }

  openEdit(rule: DoormanRule): void {
    this.editingId.set(rule.id);
    this.editName = rule.name;
    this.editEnabled = rule.enabled;
    this.editCombine = rule.combine === 'or' ? 'or' : 'and';
    this.editRiskLevel = rule.riskLevel;
    const src = Array.isArray(rule.conditions) ? rule.conditions : [];
    this.editConditions = src.map((c) => ({
      type: c.type,
      params: this.cloneParams(c.params),
    }));
    if (this.editConditions.length === 0) {
      this.editConditions = [this.newCondition()];
    }
    this.editorError.set(null);
    this.showEditor.set(true);
  }

  closeEditor(): void {
    this.showEditor.set(false);
    this.editingId.set(null);
    this.editorError.set(null);
  }

  get isEditMode(): boolean {
    return this.editingId() !== null;
  }

  // ---- 组合方式 / 风险等级选择 ------------------------------------------
  setCombine(c: DoormanCombine): void {
    this.editCombine = c;
  }

  setRiskLevel(lv: DoormanRiskLevel): void {
    this.editRiskLevel = lv;
  }

  /** 两个条件之间的连接词(且 / 或),按当前组合方式。 */
  combineWord(combine: DoormanCombine): string {
    return combine === 'or'
      ? this.labelFor('doorman.combine.orShort', '或')
      : this.labelFor('doorman.combine.andShort', '且');
  }

  // ---- 条件增删 / 类型切换 ----------------------------------------------
  private newCondition(): EditCondition {
    const first = this.conditions()[0];
    const type = first?.type ?? '';
    return { type, params: this.defaultParams(this.fieldsForType(type)) };
  }

  addCondition(): void {
    this.editConditions = [...this.editConditions, this.newCondition()];
  }

  removeCondition(index: number): void {
    this.editConditions = this.editConditions.filter((_, i) => i !== index);
  }

  onConditionTypeChange(cond: EditCondition): void {
    cond.params = this.defaultParams(this.fieldsForType(cond.type));
  }

  fieldsForType(type: string): DoormanFieldSchema[] {
    return this.findKind(this.conditions(), type)?.fields ?? [];
  }

  private findKind(list: DoormanKind[], type: string): DoormanKind | undefined {
    return list.find((k) => k.type === type);
  }

  private defaultParams(fields: DoormanFieldSchema[]): DoormanParams {
    const out: DoormanParams = {};
    for (const f of fields) {
      switch (f.type) {
        case 'string_list':
        case 'int_list':
          out[f.key] = [];
          break;
        case 'int':
          out[f.key] = null;
          break;
        case 'select':
          out[f.key] = '';
          break;
        default:
          out[f.key] = '';
      }
    }
    return out;
  }

  private cloneParams(params: DoormanParams | null | undefined): DoormanParams {
    if (!params) {
      return {};
    }
    // 结构化拷贝,避免直接改到列表里的原对象。
    return JSON.parse(JSON.stringify(params));
  }

  // ---- 动态列表字段(string_list / int_list)增删 ----------------------
  listItems(params: DoormanParams, key: string): unknown[] {
    const v = params[key];
    if (!Array.isArray(v)) {
      params[key] = [];
      return params[key] as unknown[];
    }
    return v;
  }

  addListItem(params: DoormanParams, field: DoormanFieldSchema): void {
    const arr = this.listItems(params, field.key);
    arr.push(field.type === 'int_list' ? null : '');
  }

  removeListItem(params: DoormanParams, key: string, index: number): void {
    const arr = this.listItems(params, key);
    arr.splice(index, 1);
  }

  // ngModel 需要 trackBy 才能稳定编辑列表项(否则每次输入丢焦点)。
  trackByIndex(index: number): number {
    return index;
  }

  // ---- select 候选项归一化 ----------------------------------------------
  optionValue(opt: DoormanFieldOption): string {
    return typeof opt === 'string' ? opt : opt.value;
  }

  optionLabel(opt: DoormanFieldOption): string {
    if (typeof opt === 'string') {
      return opt;
    }
    if (opt.labelKey) {
      return this.labelFor(opt.labelKey, opt.label ?? opt.value);
    }
    return opt.label ?? opt.value;
  }

  // ---- 标签解析:有 i18n 文案就用,否则回退到给定的原始串 --------------------
  labelFor(key: string | undefined, fallback: string): string {
    if (!key || !this.translate) {
      return fallback;
    }
    const t = this.translate.instant(key);
    // ngx-translate 未命中时会原样返回 key;此时用 fallback。
    return t && t !== key ? t : fallback;
  }

  kindLabel(type: string): string {
    // 优先 kind.labelKey → 约定的 doorman.kind.<type> → 原始 type。
    const kind = this.findKind(this.conditions(), type);
    return this.labelFor(kind?.labelKey, this.labelFor(`doorman.kind.${type}`, type));
  }

  fieldLabel(field: DoormanFieldSchema): string {
    return this.labelFor(field.labelKey, field.key);
  }

  riskLabel(level: DoormanRiskLevel): string {
    return this.labelFor(`doorman.risk.${level}`, level);
  }

  // ---- 风险等级 → pill / 选择按钮配色(严格对齐 web-admin,不新造颜色) ------
  //  none=slate(灰) · low=green · medium=amber · high=rose · critical=red
  riskPillClass(level: DoormanRiskLevel): string {
    switch (level) {
      case 'critical':
        return 'bg-red-100 text-red-800 border border-red-200';
      case 'high':
        return 'bg-rose-100 text-rose-800 border border-rose-200';
      case 'medium':
        return 'bg-amber-100 text-amber-800 border border-amber-200';
      case 'low':
        return 'bg-green-100 text-green-800 border border-green-200';
      case 'none':
      default:
        return 'bg-slate-100 text-slate-700';
    }
  }

  riskButtonClass(level: DoormanRiskLevel, active: boolean): string {
    if (!active) {
      return 'border-gray-300 bg-white text-gray-500 hover:bg-gray-50';
    }
    switch (level) {
      case 'critical':
        return 'border-red-400 bg-red-100 text-red-800';
      case 'high':
        return 'border-rose-400 bg-rose-100 text-rose-800';
      case 'medium':
        return 'border-amber-400 bg-amber-100 text-amber-800';
      case 'low':
        return 'border-green-400 bg-green-100 text-green-800';
      case 'none':
      default:
        return 'border-slate-400 bg-slate-100 text-slate-700';
    }
  }

  // ---- 列表里条件摘要(chips 的值) -------------------------------------
  summarizeParams(params: DoormanParams | null | undefined): string {
    if (!params) {
      return '';
    }
    const entries = Object.entries(params);
    if (entries.length === 0) {
      return '';
    }
    return entries.map(([k, v]) => `${k}: ${this.fmtVal(v)}`).join('; ');
  }

  private fmtVal(v: unknown): string {
    if (Array.isArray(v)) {
      return v.map((x) => this.fmtScalar(x)).join(', ');
    }
    return this.fmtScalar(v);
  }

  private fmtScalar(v: unknown): string {
    if (v === '') {
      return '""';
    }
    if (v === null || v === undefined) {
      return '∅';
    }
    return String(v);
  }

  formatDate(v: string | null | undefined): string {
    return formatDate(v);
  }

  // ---- 保存(新增 / 编辑) ------------------------------------------------
  save(): void {
    if (this.submitting()) {
      return;
    }
    const validationError = this.validate();
    if (validationError) {
      this.editorError.set(validationError);
      return;
    }

    const body: SaveDoormanRuleRequest = {
      scope: this.activeScope(),
      name: this.editName.trim(),
      combine: this.editCombine,
      riskLevel: this.editRiskLevel,
      enabled: this.editEnabled,
      conditions: this.editConditions.map((c) => this.toConditionDTO(c)),
    };

    this.submitting.set(true);
    this.editorError.set(null);

    const id = this.editingId();
    const req$ = id === null ? this.api.createRule(body) : this.api.updateRule(id, body);
    req$.subscribe({
      next: () => {
        this.submitting.set(false);
        this.closeEditor();
        this.reload();
      },
      error: (err) => {
        this.submitting.set(false);
        // 后端参数校验非法 → 400 + { message },展示出来。
        this.editorError.set(this.errMsg(err, 'doorman.errors.saveFailed'));
      },
    });
  }

  /** 把一条编辑态条件收敛成请求体条件;无字段的条件(如 asn_hosting)不带 params。 */
  private toConditionDTO(cond: EditCondition): ConditionDTO {
    const fields = this.fieldsForType(cond.type);
    if (fields.length === 0) {
      return { type: cond.type };
    }
    return { type: cond.type, params: this.coerceParams(cond.params, fields) };
  }

  private validate(): string | null {
    if (!this.editName.trim()) {
      return this.labelFor('doorman.errors.nameRequired', '请填写规则名称');
    }
    if (this.editConditions.length === 0) {
      return this.labelFor('doorman.errors.conditionsRequired', '请至少添加一个条件');
    }
    for (const cond of this.editConditions) {
      if (!cond.type) {
        return this.labelFor('doorman.errors.conditionTypeRequired', '请选择条件类别');
      }
      const missing = this.firstMissingRequired(cond.params, this.fieldsForType(cond.type));
      if (missing) {
        return `${this.fieldLabel(missing)}: ${this.labelFor('doorman.errors.fieldRequired', '此项为必填')}`;
      }
    }
    if (!this.editRiskLevel) {
      return this.labelFor('doorman.errors.riskLevelRequired', '请选择风险等级');
    }
    return null;
  }

  private firstMissingRequired(
    params: DoormanParams,
    fields: DoormanFieldSchema[],
  ): DoormanFieldSchema | null {
    for (const f of fields) {
      if (!f.required) {
        continue;
      }
      const v = params[f.key];
      if (f.type === 'string_list' || f.type === 'int_list') {
        if (!Array.isArray(v) || v.length === 0) {
          return f;
        }
      } else if (v === '' || v === null || v === undefined) {
        return f;
      }
    }
    return null;
  }

  // 按字段类型把值收敛成后端期望的 JSON 类型(int/int_list → number)。
  private coerceParams(params: DoormanParams, fields: DoormanFieldSchema[]): DoormanParams {
    const out: DoormanParams = {};
    for (const f of fields) {
      const v = params[f.key];
      switch (f.type) {
        case 'int':
          out[f.key] = v === '' || v === null || v === undefined ? null : Number(v);
          break;
        case 'int_list':
          out[f.key] = Array.isArray(v) ? v.map((x) => Number(x)) : [];
          break;
        case 'string_list':
          out[f.key] = Array.isArray(v) ? v.map((x) => String(x)) : [];
          break;
        default:
          out[f.key] = v ?? '';
      }
    }
    return out;
  }

  // ---- 启停(改 enabled) ------------------------------------------------
  toggleEnabled(rule: DoormanRule): void {
    const body: SaveDoormanRuleRequest = {
      scope: rule.scope,
      name: rule.name,
      combine: rule.combine,
      riskLevel: rule.riskLevel,
      conditions: rule.conditions,
      enabled: !rule.enabled,
    };
    this.api.updateRule(rule.id, body).subscribe({
      next: () => this.reload(),
      error: (err) => this.listError.set(this.errMsg(err, 'doorman.errors.toggleFailed')),
    });
  }

  // ---- 删除 --------------------------------------------------------------
  askDelete(rule: DoormanRule): void {
    this.ruleToDelete.set(rule);
  }

  cancelDelete(): void {
    this.ruleToDelete.set(null);
  }

  confirmDelete(): void {
    const rule = this.ruleToDelete();
    if (!rule) {
      return;
    }
    this.api.deleteRule(rule.id).subscribe({
      next: () => {
        this.ruleToDelete.set(null);
        this.reload();
      },
      error: (err) => {
        this.ruleToDelete.set(null);
        this.listError.set(this.errMsg(err, 'doorman.errors.deleteFailed'));
      },
    });
  }

  // ---- 错误信息提取 ------------------------------------------------------
  private errMsg(err: unknown, fallbackKey: string): string {
    if (err instanceof TimeoutError) {
      return this.labelFor('doorman.common.timeout', '请求超时,请重试');
    }
    if (err instanceof HttpErrorResponse) {
      const msg = (err.error as { message?: string } | null)?.message;
      if (msg) {
        return msg;
      }
    }
    return this.labelFor(fallbackKey, '操作失败');
  }
}
