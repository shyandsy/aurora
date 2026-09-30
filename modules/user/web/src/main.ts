import { initFederation } from '@angular-architects/native-federation';

// Native Federation remote 初始化:先建立共享作用域(shared singletons: Angular/RxJS 等),
// 再异步引导真正的应用(bootstrap.ts)。作为 remote,initFederation() 不需要 remote map。
initFederation()
  .catch((err) => console.error(err))
  .then(() => import('./bootstrap'))
  .catch((err) => console.error(err));
