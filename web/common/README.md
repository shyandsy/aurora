# web/common —— 前端共享库(通用 Angular 件,一份共用)

aurora 的**前端顶层共享库**:业务无关的通用 Angular 工具/模型/组件,供**各模块/特性前端**(`modules/*/web`、`feature/*/web`)复用。和后端顶层共享库(`middleware`/`encryption`/`types`)一个定位——通用件收口到顶层一份,模块只放自己的东西,别各自复制。

## 有什么

```
web/common/
├ utils/date.util.ts      时间格式化(统一北京时区等)
├ utils/bytes.util.ts     字节/流量单位格式化
├ models/common.dto.ts    通用 DTO(分页等)
└ components/confirm-dialog/   通用确认弹窗组件
```

全**中立**:不含任何业务名/品牌/域名。将来别的通用前端件(新 util、新共享组件)也进这里。

## 怎么用(`@common/*` 别名)

消费前端在自己的 tsconfig paths 里把 `@common/*` 指到本目录(相对路径),然后:

```ts
import { formatDate } from '@common/utils/date.util';
import { ConfirmDialogComponent } from '@common/components/confirm-dialog/confirm-dialog.component';
```

- **别把这些文件复制进你的前端**——各拷各建必漂移(前车之鉴:`feature/doorman/web` 早期自带了一份 `date.util`,已和这里漂移)。统一 import 本库。
- 本库是**源码**;随 `modules/*/web` / `feature/*/web` 一起被消费方构建进各自的 remote(aurora 不构建前端)。
- 改动须复查中立性(不得引入业务名/域名)。

## 消费者(现状)

homeserver 的 admin / customer / user 三个前端都用它(同一份)。收口到 aurora 后,各项目 `sync-aurora` 即拿到同一份,改一处全同步。
