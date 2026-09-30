const { withNativeFederation, shareAll } = require('@angular-architects/native-federation/config');

/**
 * web/user = Native Federation remote(用户中心)。
 *
 * 暴露 `./Routes` = 用户中心整棵路由子树(见 features/user-center/remote-routes.ts):
 * 宿主 web/admin 用 loadRemoteModule('user','./Routes') 把它挂进自己内容区的 router-outlet,
 * 原生渲染用户/角色/权限页(顶部 tab)。**暴露的是路由,不是 web/user 的 AppComponent**,
 * 故 web/user 自己的 sidebar/topbar/壳不会被宿主加载(无双侧栏);web/user 独立运行时仍用自己的
 * AppComponent+sidebar 包这些路由(见 src/app/app.routes.ts)。
 * `ng build`(NF builder)在 dist/user/browser/ 下产出 remoteEntry.json(含 `./Routes` 的 chunk 映射)。
 */
module.exports = withNativeFederation({
  name: 'user',

  exposes: {
    './Routes': './src/app/features/user-center/remote-routes.ts',
  },

  shared: {
    // @angular/*、rxjs、@ngx-translate/core 等 package.json 依赖全部共享为单例(singleton:true),
    // 宿主与 remote 共用同一份框架 + 同一个 TranslateService 实例(i18n 词典合并靠这个前提)。
    ...shareAll({ singleton: true, strictVersion: true, requiredVersion: 'auto' }),
  },

  skip: [
    'rxjs/ajax',
    'rxjs/fetch',
    'rxjs/testing',
    'rxjs/webSocket',
    // qrcode → pngjs 依赖 node 内建(util/stream),不能作为浏览器共享 ESM 单独打包;
    // 跳过 federation 共享,让它像普通依赖一样被各自的 bundle 正常打包(与开 NF 前一致)。
    'qrcode',
    // 注:@common/* 是 tsconfig 路径别名(指向 web/common 源码),不在这里处理:
    //   - 不能放进 skip(会被标成 external 运行时模块、无 federated 提供者 → 运行时 404);
    //   - 不能放进 host/remote 根 tsconfig 的 paths(NF 的 sheriff 依赖裁剪会顺着 @common 走到
    //     项目根外的 web/common → 抛 "outside of root directory" 构建失败)。
    // 解决办法见 src/app/tsconfig.json:给暴露产物(federation artefacts)的 esbuild 就近提供 @common
    // 解析,而根 tsconfig 保持干净、sheriff 看不到 @common。@common 由此打进各自 bundle(admin/user 各一份)。
  ],

  features: {
    ignoreUnusedDeps: true,
  },
});
