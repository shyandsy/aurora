# 分支 / 提交规范(shyandsy 各仓统一 · 单一源)

> 这是跨项目统一约定。aurora 是单一源;各消费项目照此固化到自己仓(见下「怎么落到各项目」)。

## 分支命名

- **默认只用三种前缀:`feature/` `bugfix/` `docs/`。**
- **维护 / 依赖升级 / 杂务一律归 `feature/`**(例:升级依赖 → `feature/bump-xxx`)。
- **不要**用 `chore/` `fix/` `feat/` `ops/` `refactor/` 等其它前缀——保持收敛,检索/自动化才稳。
- **绝不**强推(force-push)`main`/`master`;合并一律走 PR。

> **按项目放宽**:个别仓确实需要额外前缀(例:`research/` 做 WIP/实验),在该仓的
> `.githooks/branch-prefixes` 里列出允许前缀即可(每行一个、不带斜杠,`#` 注释、空行忽略);
> 该文件缺省或为空 → 回退默认三前缀。**改允许集改这个文件,别手改 `pre-push` 脚本**(脚本是 aurora 托管副本)。

## 提交 / PR

- commit 与 PR 描述里**不写** AI 工具署名(如「🤖 Generated with …」/「Co-Authored-By: …」)。

## 为什么要有机械守卫(而不是只写文档)

文档 / 记忆是「被动」的——只在有人去读时才起作用,靠不住。所以除了写下来,还要一层
**push 时机械拦截**的 `pre-push` 钩子:不符前缀的分支名直接被拒,不依赖任何人(或 agent)记得。

> 注:skill 不适合干这个——skill 只在 agent 主动为某个任务选用时才触发,而「给分支起名」是
> 顺手的小动作、不是一个会去选 skill 的任务。规范这类事要「常驻 / 动手那刻就拦」,不是按需。

## 怎么落到各项目(消费方式)

aurora 持有单一源:`scripts/git-hooks/pre-push`。各项目:

1. 把该钩子拷进自己仓的 `.githooks/pre-push`(托管副本,别手改;改规范去 aurora 改后重拷),`chmod +x`。
   需要额外前缀就加 `.githooks/branch-prefixes`(见上)。
2. 每个 clone 跑一次:`git config core.hooksPath .githooks`(建议包一个 `make dev-setup`)。
3. 在仓库根 `CLAUDE.md` 顶部写明上面的分支/提交规范——它每个 session 自动加载,
   所有 agent 开局即见(这是「被看到」那层,和钩子的「被拦截」那层互补)。
4. **(推荐)配一个 CI 分支名检查**——pre-push 是本地钩子,能被 `--no-verify` 绕、也要每个 clone
   跑 `dev-setup` 才生效。再加一个 PR 上的 CI job(读**同一份** `.githooks/branch-prefixes`,
   head 分支名不符就 fail),才是不可绕、零设置的硬门。三层合起来:**被看到(CLAUDE.md)+
   被拦截(钩子)+ 被强制(CI)**。

> 一个小钩子文件的副本 ≠ vendor aurora;和 skill 分发一样,是「带一个独立小工具」。
