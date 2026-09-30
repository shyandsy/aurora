# modules/user/web —— 用户中心前端(共享源,构建一份版本化 remote)

这是 `modules/user`(后台用户中心)的**配套前端**:一个完整的 Angular **Native Federation remote** SPA(登录壳 + 账号/角色/权限/2FA/会话/微服务 token/被锁列表页面),作为 remote 暴露 `./Routes`,由消费项目的后台壳(host)在运行时加载、渲染进宿主内容区。

和后端同源共置(`modules/user/` 正下方),扫一眼就知道 user 模块是**全栈**的——避免"前端在别的仓 → 被忘"。

## aurora 只**存源码**,不构建

aurora 是框架仓,不跑 `ng build`。这里放的是**源码**;由消费方**从这份源构建出一份版本化的共享 remote**(`/user/vN/remoteEntry.json`),各接入项目的后台壳 `pin` 到某版本加载。

## 消费 = 一份共享 remote,**别各项目各拷**

前车之鉴:doorman 现在是"各项目把组件拷进自己的 remote app",实测已和 aurora 源**逐字节漂移**——各拷各建必 drift,就是"重写"的变种。user 是整个 SPA,更要避开:

- **对**:从本目录源构建**一份**共享 remote → 各项目壳 pin 版本加载;升级 = 壳显式 bump manifest(不重构建 host、坏了不连累别人)。
- **错**:每个项目把 `src/app` 拷进自己的 app 各自构建。

host 集成大致三步(声明 remote → 加懒加载路由 `loadRemoteModule('user','./Routes')` → 提供壳布局);详见设计稿 [§4.3 前端](../../../doc/proposals/shared-user-center.md)。

## 中立(硬性)

本目录**不含任何业务名/品牌/域名/私有镜像仓**(白标中性);渠道差异(品牌、apiBaseUrl 等)全走消费侧 env/挂载,不进这里的代码/模板/类名。改动后须复查中立性。

## 结构

```
modules/user/web/
├ angular.json  package.json  federation.config.js  tsconfig*.json   ← remote 构建配置(暴露 ./Routes)
├ public/                                                            ← 静态资源
└ src/app/
   ├ shell/        登录壳 + 布局
   ├ pages/        各功能页(账号/角色/权限/会话/被锁列表/微服务 token…)
   ├ features/     特性模块(user-center 路由子树 remote-routes.ts 在此)
   ├ core/         核心(路由/拦截器/守卫…)
   └ shared/       共享(models/services/utils/组件)
```

> 构建/本地开发命令见 `federation.config.js` 注释与 `package.json` scripts;消费方接入见设计稿 §4.3。
