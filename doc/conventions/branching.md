# 分支 / 提交规范(shyandsy 各仓统一 · 单一源)

> 这是跨项目统一约定。aurora 是单一源;各消费项目照此固化到自己仓(见下「怎么落到各项目」)。

## 分支命名

- **只用三种前缀:`feature/` `bugfix/` `docs/`。**
- **维护 / 依赖升级 / 杂务一律归 `feature/`**(例:升级依赖 → `feature/bump-xxx`)。
- **不要**用 `chore/` `fix/` `feat/` `ops/` `refactor/` 等其它前缀——保持三种,检索/自动化才稳。
- **绝不**强推(force-push)`main`/`master`;合并一律走 PR。

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
2. 每个 clone 跑一次:`git config core.hooksPath .githooks`(建议包一个 `make dev-setup`)。
3. 在仓库根 `CLAUDE.md` 顶部写明上面的分支/提交规范——它每个 session 自动加载,
   所有 agent 开局即见(这是「被看到」那层,和钩子的「被拦截」那层互补)。

> 一个小钩子文件的副本 ≠ vendor aurora;和 skill 分发一样,是「带一个独立小工具」。
