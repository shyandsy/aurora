import { Injectable, inject } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';
import { timeout } from 'rxjs/operators';
import {
  KindsResponse,
  DoormanRule,
  SaveDoormanRuleRequest,
  DeleteDoormanRuleResponse,
  PolicyKindsResponse,
  ActionPolicy,
  StatsResponse,
  DecisionRow,
} from '../models/doorman.dto';

// ⚠️ 接入方按自己 app 改这两处(原是框架无关的默认值,不再依赖某个 app 的 environment):
//   - API_TIMEOUT_MS:HTTP 超时(毫秒)。你的 app 有统一超时就换成它。
//   - TOKEN_STORAGE_KEY:从 localStorage 读 access token 的 key。你的 app 存 token 的 key 是什么就填什么;
//     若你的 HttpClient 已有 auth 拦截器统一加 Authorization,可把 getHeaders 里的 token 逻辑整段删掉。
const API_TIMEOUT_MS = 15000;
const TOKEN_STORAGE_KEY = 'access_token';

/**
 * doorman 规则配置的瘦 HTTP 访问层(单一出口)。
 *
 * baseUrl 不走注入 token,而是**由挂载它的 DoormanConfigComponent 从 @Input `apiBase` 赋值**
 * (见 doorman-config.component.ts)。原因:doorman 作为 federation remote 暴露的是**组件**
 * (不是路由),apiBase 是宿主运行时通过组件输入传进来的,拿不到编译期/路由级的注入 token。
 * 故本服务在组件级 providers 里 provide,再由组件把 apiBase 写进 baseUrl。
 *
 * 走的是 Angular HttpClient(独立运行=本 app 的 provideHttpClient;federated=宿主共享单例,宿主拦截器照常生效)。
 * getHeaders 主动带 Authorization(从 localStorage 读 TOKEN_STORAGE_KEY);若你的 HttpClient 已有 auth 拦截器
 * 统一加 Authorization,把 getHeaders 里的 token 逻辑删掉即可。
 *
 * 错误**不在这里吞**:组件需要拿到 400 的 { message } 内联展示,故让 HttpErrorResponse 原样抛给调用方。
 */
@Injectable()
export class DoormanApi {
  private readonly http = inject(HttpClient);

  /** 后端 doorman 管理 API 的基路径,如 `/api/admin/v1/doorman`。由组件从 @Input apiBase 赋值。 */
  baseUrl = '';

  private getHeaders(): HttpHeaders {
    let headers = new HttpHeaders({ 'Content-Type': 'application/json' });
    let token: string | null = null;
    try {
      token = localStorage.getItem(TOKEN_STORAGE_KEY);
    } catch {
      token = null;
    }
    if (token) {
      headers = headers.set('Authorization', `Bearer ${token}`);
    }
    return headers;
  }

  private buildUrl(endpoint: string): string {
    if (endpoint.startsWith('http://') || endpoint.startsWith('https://')) {
      return endpoint;
    }
    // baseUrl 可能带或不带尾斜杠,endpoint 以 / 开头;去掉 baseUrl 尾部多余斜杠避免出现 `//`。
    const base = this.baseUrl.replace(/\/+$/, '');
    return `${base}${endpoint}`;
  }

  /** 拿所有条件/动作类别及其字段 schema。 */
  getKinds(): Observable<KindsResponse> {
    return this.http
      .get<KindsResponse>(this.buildUrl('/kinds'), { headers: this.getHeaders() })
      .pipe(timeout(API_TIMEOUT_MS));
  }

  /** 列出某 scope 下的规则。 */
  getRules(scope: string): Observable<DoormanRule[]> {
    const q = new URLSearchParams({ scope }).toString();
    return this.http
      .get<DoormanRule[]>(this.buildUrl(`/rules?${q}`), { headers: this.getHeaders() })
      .pipe(timeout(API_TIMEOUT_MS));
  }

  /** 新增规则(201 返回创建后的 rule;400 → { message })。 */
  createRule(req: SaveDoormanRuleRequest): Observable<DoormanRule> {
    return this.http
      .post<DoormanRule>(this.buildUrl('/rules'), req, { headers: this.getHeaders() })
      .pipe(timeout(API_TIMEOUT_MS));
  }

  /** 编辑规则(200 返回更新后的 rule;400 → { message })。 */
  updateRule(id: number, req: SaveDoormanRuleRequest): Observable<DoormanRule> {
    return this.http
      .put<DoormanRule>(this.buildUrl(`/rules/${id}`), req, { headers: this.getHeaders() })
      .pipe(timeout(API_TIMEOUT_MS));
  }

  /** 删除规则。 */
  deleteRule(id: number): Observable<DeleteDoormanRuleResponse> {
    return this.http
      .delete<DeleteDoormanRuleResponse>(this.buildUrl(`/rules/${id}`), { headers: this.getHeaders() })
      .pipe(timeout(API_TIMEOUT_MS));
  }

  // ── 「风险等级 → 动作」策略 ──

  /** 拿配置「风险→动作」所需选项:风险等级枚举 + 该 scope 可选动作名。 */
  getPolicyKinds(scope: string): Observable<PolicyKindsResponse> {
    const q = new URLSearchParams({ scope }).toString();
    return this.http
      .get<PolicyKindsResponse>(this.buildUrl(`/policy/kinds?${q}`), { headers: this.getHeaders() })
      .pipe(timeout(API_TIMEOUT_MS));
  }

  /** 拿某 scope 当前的「风险等级→动作名」映射。 */
  getPolicy(scope: string): Observable<ActionPolicy> {
    const q = new URLSearchParams({ scope }).toString();
    return this.http
      .get<ActionPolicy>(this.buildUrl(`/policy?${q}`), { headers: this.getHeaders() })
      .pipe(timeout(API_TIMEOUT_MS));
  }

  /** 整体覆盖某 scope 的策略(400 → { message })。 */
  setPolicy(scope: string, mapping: ActionPolicy): Observable<{ ok: boolean }> {
    const q = new URLSearchParams({ scope }).toString();
    return this.http
      .put<{ ok: boolean }>(this.buildUrl(`/policy?${q}`), mapping, { headers: this.getHeaders() })
      .pipe(timeout(API_TIMEOUT_MS));
  }

  /** 决策统计(近 days 天;各风险等级命中数 / 各动作数)。 */
  getStats(scope: string, days: number): Observable<StatsResponse> {
    const q = new URLSearchParams({ scope, days: String(days) }).toString();
    return this.http
      .get<StatsResponse>(this.buildUrl(`/stats?${q}`), { headers: this.getHeaders() })
      .pipe(timeout(API_TIMEOUT_MS));
  }

  /** 决策明细(id 倒序;before=上一页最后一条 id 做游标翻页,0=首页)。 */
  getDecisions(scope: string, limit: number, before: number): Observable<DecisionRow[]> {
    const q = new URLSearchParams({ scope, limit: String(limit), before: String(before) }).toString();
    return this.http
      .get<DecisionRow[]>(this.buildUrl(`/decisions?${q}`), { headers: this.getHeaders() })
      .pipe(timeout(API_TIMEOUT_MS));
  }
}
