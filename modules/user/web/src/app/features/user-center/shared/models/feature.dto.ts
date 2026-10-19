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
  created: string;
  modified: string;
}

