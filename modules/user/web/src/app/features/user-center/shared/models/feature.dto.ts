/**
 * Feature DTOs
 */

export interface Feature {
  id: number;
  name: string;
  // 分档维度(后端 20260816120000 迁移新增)。kind=api|ui;action=read|write|view|operate;module=归属页/资源。
  // 不参与鉴权,供页面级 view/operate 授权 UI 分组/分档用。
  module?: string;
  action?: string;
  kind?: string;
  // 「可授权页目录」展示元数据(仅 ui.page.* 行有值,由各项目的 data 套迁移 seed,后端 label_i18n/sort_order 列)。
  // labelI18n:多语言页名 { 'zh-CN': '客户', 'en': 'Customers', ... },前端按当前语言取,不写死语言集。
  // sortOrder:页目录展示顺序。
  labelI18n?: Record<string, string>;
  sortOrder?: number;
  created: string;
  modified: string;
}

