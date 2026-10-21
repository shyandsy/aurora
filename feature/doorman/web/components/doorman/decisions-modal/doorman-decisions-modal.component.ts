import { Component, inject, input, output, signal, computed, OnInit, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { DoormanApi } from '../shared/services/doorman-api';
import { DecisionRow, StatsResponse, DoormanRiskLevel } from '../shared/models/doorman.dto';
import { DOORMAN_I18N } from '../i18n';
import { formatDate } from '@common/utils/date.util';

/** 风险等级分布的一段(固定顺序 critical→none;配色对齐 doorman 规则页)。 */
interface RiskSegment {
  level: DoormanRiskLevel;
  count: number;
  /** 堆叠条段底色 class。 */
  barClass: string;
  /** 图例圆点色 class。 */
  dotClass: string;
  /** 占 total 的百分比(0~100),用作堆叠条段宽。 */
  pct: number;
}

/** KPI 行一格:合计 + 各动作计数(动作名走 doorman.action.*,原始名兜底)。 */
interface ActionKpi {
  action: string;
  count: number;
  /** 格配色 class(由宿主传的 action→配色键决定;组件只认配色键,不认业务动作,保持中立)。 */
  card: string;
  label: string;
  num: string;
}

/** KPI 配色键 → class。宿主按业务语义把 action 映射到某个键(如 block→rose),组件只认这几个中性键。 */
const KPI_TONES: Record<string, { card: string; label: string; num: string }> = {
  rose: { card: 'border-rose-100 bg-rose-50', label: 'text-rose-600', num: 'text-rose-700' },
  amber: { card: 'border-amber-100 bg-amber-50', label: 'text-amber-600', num: 'text-amber-700' },
  green: { card: 'border-green-100 bg-green-50', label: 'text-green-600', num: 'text-green-700' },
  indigo: { card: 'border-indigo-100 bg-indigo-50', label: 'text-indigo-600', num: 'text-indigo-700' },
  slate: { card: 'border-gray-200 bg-white', label: 'text-gray-500', num: 'text-gray-900' },
};

/**
 * 决策明细弹窗 —— doorman(看门人)**中立**风险评估器的共享组件,Native Federation 暴露为 `./Decisions`。
 *
 * 一块抛光的浮层:固定头 → 固定紧凑统计条(KPI 一排 + 风险分布 | 漏斗 并排)→ 单行台账表 → 固定脚
 * (PII 说明 + 页码分页)。
 *
 * 布局约定(与客户页弹窗**统一**):每页固定 PAGE_SIZE=10 条,不按窗口高度动态测量;表格**永远渲染 10 个
 * 行槽**,本页不足 10 条时补空行 → 面板高度恒定,翻到最后一页(数据不满)也不坍缩、表格下方不留空白。
 * 面板高度由内容撑(恒 10 行槽即恒定),不写死像素;窗口过矮时 max-height 内部滚动兜底。
 *
 * **中立性**:只认 doorman 自己的概念 —— scope / riskLevel(none..critical)/ action / decision / funnel。
 * 动作名一律按 i18n key `doorman.action.<name>` 渲染(name 来自数据),没有对应文案就显示原始 name;
 * 任何业务动作中文词由**宿主项目字典**提供,不写死在本组件。KPI 行 = 合计 + 各 action 计数(通用),
 * 不写死某几个动作。
 *
 * 宿主用 `loadRemoteModule('doorman','./Decisions').then(m => m.DoormanDecisionsModalComponent)` 拿到本组件,
 * 绑定 `[apiBase]`(后端 doorman 管理 API 基路径)与 `[scope]`(''=全量台账),监听 `(close)`。
 */
@Component({
  selector: 'app-doorman-decisions-modal',
  standalone: true,
  imports: [CommonModule, TranslateModule],
  providers: [DoormanApi],
  templateUrl: './doorman-decisions-modal.component.html',
  styleUrl: './doorman-decisions-modal.component.css',
})
export class DoormanDecisionsModalComponent implements OnInit {
  private readonly api = inject(DoormanApi);
  private readonly translate = inject(TranslateService, { optional: true });
  private readonly cdr = inject(ChangeDetectorRef);

  // ---- 宿主输入 ----------------------------------------------------------
  /** 后端 doorman 管理 API 的基路径,如 `/api/admin/v1/doorman`。 */
  apiBase = input<string>('');
  /** 作用域:''=全量台账;传某 scope(如 "register")=只看该场景。 */
  scope = input<string>('');
  /** 可选标题 / 副标题覆盖(不传走 doorman.decisions.title/subtitle)。 */
  title = input<string>('');
  subtitle = input<string>('');
  /** 统计窗口天数(KPI / 分布 / 漏斗都用它)。 */
  days = input<number>(30);
  /** 可选:动作名 → 配色键(rose/amber/green/indigo/slate)。宿主按业务语义传
   * (如 {block:'rose', email_verify:'amber', allow:'green'});组件只认配色键、不认业务动作,保持中立。
   * 未映射的动作 → slate(中性白底)。 */
  actionColors = input<Record<string, string>>({});
  /** 遮罩左起点(CSS 长度)。宿主有侧栏时传其宽度(如 '16rem')→ 遮罩从侧栏右缘起、不压侧栏、不跟它抢 z;
   * 默认 '0'=全屏(无侧栏宿主)。照 #759 的证明有效做法,避免被高 z 侧栏盖住。 */
  insetLeft = input<string>('0');

  /** 关闭弹窗。 */
  close = output<void>();

  /** 每页固定条数(与客户页弹窗统一;表格恒渲染这么多行槽,不足补空行)。 */
  private static readonly PAGE_SIZE = 10;
  /** 风险等级固定渲染顺序(高→低),对齐设计图图例。 */
  private static readonly RISK_ORDER: DoormanRiskLevel[] = [
    'critical',
    'high',
    'medium',
    'low',
    'none',
  ];

  // ---- 数据态 ------------------------------------------------------------
  /** 当前页记录;null = 加载失败(给错误占位)。 */
  decisions = signal<DecisionRow[] | null>(null);
  /** 统计(KPI / 分布 / 漏斗);null = 未加载 / 失败,统计条回落 0。 */
  stats = signal<StatsResponse | null>(null);
  /** 数据加载中(含翻页)。 */
  loading = signal(false);
  /** 加载失败(展示占位,不崩)。 */
  error = signal(false);

  /** 总条数(页码分页)。 */
  total = signal(0);
  /** 当前页码(从 1 起)。 */
  currentPage = signal(1);
  /** 每页条数(固定 10)。 */
  pageSize = signal(DoormanDecisionsModalComponent.PAGE_SIZE);

  /** 总页数(按 total/pageSize 现算)。 */
  totalPages = computed(() => {
    const ps = this.pageSize();
    return ps > 0 ? Math.ceil(this.total() / ps) : 0;
  });
  hasNext = computed(() => this.currentPage() < this.totalPages());
  hasPrev = computed(() => this.currentPage() > 1);

  /** 空行补齐数量:本页不足 PAGE_SIZE 时补空行槽,保证面板高度恒定、最后一页不坍缩、不留空白。 */
  fillerRows = computed<number[]>(() => {
    const list = this.decisions();
    const have = Array.isArray(list) ? list.length : 0;
    const pad = this.pageSize() - have;
    return pad > 0 ? Array.from({ length: pad }, (_, i) => i) : [];
  });

  // ---- 统计条:KPI(合计 + 各动作,通用) ------------------------------
  kpiTotal = computed(() => this.stats()?.total ?? 0);
  /** 各动作计数(来自 stats.byAction map);按动作名 + doorman.action.* 渲染,无硬编码动作。 */
  actionKpis = computed<ActionKpi[]>(() => {
    const by = this.stats()?.byAction ?? {};
    const colors = this.actionColors();
    return Object.keys(by).map((action) => {
      const tone = KPI_TONES[colors[action] ?? 'slate'] ?? KPI_TONES['slate'];
      return { action, count: by[action] ?? 0, card: tone.card, label: tone.label, num: tone.num };
    });
  });

  // ---- 统计条:风险等级分布(byRisk map → 固定顺序分段) ----------------
  riskSegments = computed<RiskSegment[]>(() => {
    const by = this.stats()?.byRisk ?? {};
    const total = this.stats()?.total ?? 0;
    return DoormanDecisionsModalComponent.RISK_ORDER.map((level) => {
      const count = by[level] ?? 0;
      return {
        level,
        count,
        barClass: this.riskBarClass(level),
        dotClass: this.riskBarClass(level),
        pct: total > 0 ? (count / total) * 100 : 0,
      };
    });
  });

  /**
   * 图例主项:按计数降序取前 2 的「非 none」等级(count>0)。
   * 设计把图例压在风险分布细条右侧单行,故只显主项,其余归「+N」。
   */
  riskLegendPrimary = computed<RiskSegment[]>(() => {
    const nonNone = this.riskSegments().filter((s) => s.level !== 'none' && s.count > 0);
    nonNone.sort((a, b) => b.count - a.count);
    return nonNone.slice(0, 2);
  });
  /** 图例溢出项:主项之外仍有计数的等级(含 none),按严重度降序(riskSegments 已是高→低)。 */
  riskLegendOverflow = computed<RiskSegment[]>(() => {
    const shown = new Set(this.riskLegendPrimary().map((s) => s.level));
    return this.riskSegments().filter((s) => s.count > 0 && !shown.has(s.level));
  });

  // ---- 统计条:挑战 → 解决漏斗 ------------------------------------------
  funnelChallenged = computed(() => this.stats()?.funnel?.challenged ?? 0);
  funnelResolved = computed(() => this.stats()?.funnel?.resolved ?? 0);
  /** 未解决 = 挑战 − 解决(下限 0)。 */
  funnelUnresolved = computed(() => Math.max(0, this.funnelChallenged() - this.funnelResolved()));
  /** 解决率(0~100);challenged=0 时返回 null(显示「—」)。 */
  funnelRate = computed<number | null>(() => {
    const c = this.funnelChallenged();
    return c > 0 ? (this.funnelResolved() / c) * 100 : null;
  });
  /** 有挑战记录才显示漏斗。 */
  hasFunnel = computed(() => this.funnelChallenged() > 0);

  constructor() {
    this.mergeI18n();
  }

  ngOnInit(): void {
    this.api.baseUrl = this.apiBase();
    this.loadStats();
    this.loadPage(1);
  }

  // ---- i18n:把模块三语字典深合并进 ngx-translate(弹窗可脱离配置页独立挂载,故自己也合并一次) ----
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
    this.translate.onLangChange.subscribe((e) => apply(e.lang));
  }

  /** 拉统计(KPI / 分布 / 漏斗);失败静默回落 0,不影响台账。 */
  private loadStats(): void {
    this.api.getStats(this.scope(), this.days()).subscribe({
      next: (s) => {
        this.stats.set(s);
        this.cdr.detectChanges();
      },
      error: () => {
        /* 统计 best-effort:失败不打扰台账 */
      },
    });
  }

  /** 拉指定页(页码分页),失败置 error 占位。 */
  private loadPage(page: number): void {
    this.loading.set(true);
    this.error.set(false);
    this.cdr.detectChanges();
    this.api.getDecisionsPage(this.scope(), page, this.pageSize()).subscribe({
      next: (data) => {
        this.decisions.set(Array.isArray(data?.items) ? data.items : []);
        this.total.set(data?.total ?? 0);
        this.currentPage.set(data?.page ?? page);
        this.loading.set(false);
        this.cdr.detectChanges();
      },
      error: () => {
        this.decisions.set(null);
        this.error.set(true);
        this.loading.set(false);
        this.cdr.detectChanges();
      },
    });
  }

  /** 页码分页:切到目标页。 */
  onPageChange(page: number): void {
    if (page < 1 || page > this.totalPages() || page === this.currentPage()) {
      return;
    }
    this.loadPage(page);
  }

  prevPage(): void {
    if (this.hasPrev()) {
      this.onPageChange(this.currentPage() - 1);
    }
  }

  nextPage(): void {
    if (this.hasNext()) {
      this.onPageChange(this.currentPage() + 1);
    }
  }

  // Sentinel used for the "…" gap markers in the windowed page list.
  readonly ellipsis = '…';

  /**
   * 窗口化页码列表:始终显示首页 + 末页 + 当前页附近的一小窗,其余用「…」。
   * 避免 totalPages 很大时渲染上千个按钮。返回页码与 ellipsis 混合列表。
   */
  pageWindow(): (number | string)[] {
    const total = this.totalPages();
    const current = this.currentPage();
    const delta = 2;
    if (total <= 7) {
      return Array.from({ length: total }, (_, i) => i + 1);
    }
    const pages: (number | string)[] = [1];
    const left = Math.max(2, current - delta);
    const right = Math.min(total - 1, current + delta);
    if (left > 2) {
      pages.push(this.ellipsis);
    }
    for (let i = left; i <= right; i++) {
      pages.push(i);
    }
    if (right < total - 1) {
      pages.push(this.ellipsis);
    }
    pages.push(total);
    return pages;
  }

  isPageNumber(p: number | string): p is number {
    return typeof p === 'number';
  }

  /** 列表为空(已加载但本页无记录)。 */
  isEmpty(): boolean {
    const d = this.decisions();
    return d != null && d.length === 0;
  }

  // ---- 展示 helper(全部内置,不依赖宿主共享件) ------------------------
  /** 时间格式化(走 doorman 特性自带的 @common 日期工具,与规则页一致)。 */
  fmtTime(created?: string): string {
    return created ? formatDate(created, '') : '';
  }

  /** 标签解析:有 i18n 文案就用,否则回退给定原始串。 */
  private labelFor(key: string, fallback: string): string {
    if (!this.translate) {
      return fallback;
    }
    const t = this.translate.instant(key);
    return t && t !== key ? t : fallback;
  }

  /** 弹窗标题 / 副标题:优先 @Input 覆盖,否则走 doorman.decisions.* 文案。 */
  titleText(): string {
    return this.title() || this.labelFor('doorman.decisions.title', 'Decisions');
  }
  subtitleText(): string {
    return this.subtitle() || this.labelFor('doorman.decisions.subtitle', '');
  }

  /** 动作名 → 展示文案:优先 i18n doorman.action.<name>,回退原始动作名(中立兜底)。 */
  actionLabel(action: string): string {
    if (!action) {
      return this.labelFor('doorman.decisions.action.default', '—');
    }
    return this.labelFor(`doorman.action.${action}`, action);
  }

  /** 风险等级展示名:doorman.risk.<level> 有就用,否则原始 level。 */
  riskLabel(level: string): string {
    return this.labelFor(`doorman.risk.${level}`, level);
  }

  /** 图例「+N」的 title:溢出等级逐个「名 计数」用「 · 」连接(严重度降序)。 */
  riskOverflowTitle(): string {
    return this.riskLegendOverflow()
      .map((s) => `${this.riskLabel(s.level)} ${s.count}`)
      .join(' · ');
  }

  /**
   * 风险等级徽章配色(台账 pill),严格对齐 doorman 规则页:
   * critical=red · high=rose · medium=amber · low=green · none/未知=slate。
   */
  riskLevelClass(level: string): string {
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

  /** 风险分布堆叠条段 / 图例圆点色。 */
  private riskBarClass(level: DoormanRiskLevel): string {
    switch (level) {
      case 'critical':
        return 'bg-red-500';
      case 'high':
        return 'bg-rose-500';
      case 'medium':
        return 'bg-amber-500';
      case 'low':
        return 'bg-green-500';
      case 'none':
      default:
        return 'bg-slate-300';
    }
  }

  /** 网络展示:ua · ip · country · isp · AS{asn} {asnOrg},空字段跳过,全空显示「—」。 */
  networkText(d: DecisionRow): string {
    const parts: string[] = [];
    if (d.ua) {
      parts.push(d.ua);
    }
    if (d.ip) {
      parts.push(d.ip);
    }
    if (d.country) {
      parts.push(d.country);
    }
    if (d.isp) {
      parts.push(d.isp);
    }
    if (d.asn) {
      parts.push('AS' + d.asn + (d.asnOrg ? ' ' + d.asnOrg : ''));
    }
    return parts.join(' · ') || '—';
  }

  onClose(): void {
    this.close.emit();
  }
}
