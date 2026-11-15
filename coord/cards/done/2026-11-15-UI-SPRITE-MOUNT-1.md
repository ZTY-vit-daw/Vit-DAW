# UI-SPRITE-MOUNT-1：DesignSprite 全局挂载——修复全应用图标不渲染

- 发卡：GLM 主管决策流 / 2026-11-15（用户手测观察：「发送和等待输出都没有 icon」——359 目检后续深查触发）
- 派发确认：待用户触发领取
- 验收负责人：主管决策流 + 用户目检（发送键 i-send/回复中 i-stop 可见）
- 池序 361；P1 / 0.25 天 / 依赖=无；PC；零栈（浏览器渲染面）

## 缺陷（已定位）

- `agent/webui/src/design/sprite.tsx:91` 导出 `DesignSprite()`（symbol 定义挂载组件），**全代码库无任何入口挂载它**——`<use href="#i-send">` 等 sprite 引用全部落空，**全应用所有 v5 sprite 图标渲染为空**（发送键=黑方块无图标；busy 态 i-stop 同空）。
- 连带缺陷：渲染烟测的图标对比度断言（data-graphic ≥3:1）空转——图标为零时无从检查=空过，未拦截本缺陷。
- 附带记录：应用内存在**双图标体系**——v5 组件用 sprite（TextField/MacroCard/gallery），App.tsx 工程面 composer 仍用 lucide 内联图标（`<Send>`/`<Square>`，App.tsx:7096-7106，能渲染）。本卡不强制统一，但须在回执中记录两体系清单供后续裁定。

## 目标

1. 在应用入口挂载 `DesignSprite()`（main.tsx/App 根/LauncherSurface 根——领取时锚定唯一正确位置，一次挂载全树可用）。
2. 渲染烟测图标断言去空转化：断言页面 sprite 图标元素**计数 ≥1** 且对比度检查非空集合，防回归。
3. `npm run test`（vitest）+ `npm run build` + `npm run test:render` 全绿；逐面截图（launcher+project）留工件。

## 非目标

- 图标体系统一（sprite vs lucide）——记录清单，另裁定。
- 磁带 icon 三态动效（SPEC-V5 §7.4）——后续卡候选。

## 文件域

`agent/webui/src/`（挂载点+渲染烟测断言）+ coord 工件；领取时回填精确清单。

## 验收标准

三门全绿+图标计数断言在卷+用户目检发送/停止键图标可见。

## 停止条件

挂载后出现样式冲突（sprite.css 全局泄漏）且非显式选择器可收敛→取证上交。

## 领取/回执/验收

### 领取（2026-11-15）

- owner：GLM-5.3 Flash 执行流（ZCode 免费车道量产子代理，PC / Git Bash）
- 领取时 origin/main：5957e5a2132ff7fc2b0be21b8c9af3ac73f8604b
- 主仓本地 main：5672fc39（=origin/main + 4 个 coord-only 未推提交 358/360/361/362 发卡与裁定，逐提交 show --stat 核对仅含 coord/ 变更；领取提交随其一同 fast-forward 推送）
- 主仓工作树领取时状态（非本卡文件域，不触碰不整理）：
  - tracked 改动：`CURRENT-STATE.md`、`VitApp/Workspace/Settings/Settings.xml`、`VitApp/Workspace/default_project.xml`、`coord/decisions/2026-11-11-legame-demo-experience-first.md`、`tracktion_engine`（submodule 指针，diff --stat 5 files +4464/−4421）
  - untracked：`.vit_derived/`、`.zcode/`、`agent/webui/design/*` 设计稿等
- 执行分支：port/ui-sprite-1（基线=origin/main 5957e5a2）
- worktree：D:/Vit_DAW_wt_uisprite1
- 文件域：`agent/webui/src/main.tsx`（挂载+sprite.css 导入）、`agent/webui/src/design/sprite.tsx`（如需微调，预期不动）、`agent/webui/src/gallery/render-smoke.mjs`（图标计数/非空断言+本卡工件）、coord 工件；禁入：LauncherSurface.tsx、App.tsx、launcher.css、agent/internal/**、scripts/**
- 勘察补充事实（同缺陷链，回执详述）：sprite.css 全仓仅 gallery/main.tsx 导入——真实入口缺 CSS 导入，只挂载不导入会致 .v5-sprite 裸 svg 以默认 300×150 显示；修复=挂载+导入均在 main.tsx。渲染烟测只测 gallery.html fixture 面（各面自挂 sprite），从不加载真实入口 index.html——挂载缺失因此逃逸。

### 回执（2026-11-15，执行侧自验完成，待主管验收）

- 实现 commit：**59842aa8**（port/ui-sprite-1，基线 5957e5a2；2 files +226/−1 = main.tsx + render-smoke.mjs；分支已推 origin）
- 挂载点：`agent/webui/src/main.tsx`（真实入口 index.html 的 React createRoot 根渲染处，StrictMode 内、surface 分发之前一次挂载——launcher/project/main/full/workbench/trajectory 全 surface 生效）。卡面候选点 App.tsx/LauncherSurface.tsx 未采用且未改动。
- 连带必要修复（同缺陷链，域内）：`design/sprite.css` 此前全仓仅 gallery/main.tsx 导入——真实入口只挂载不导入会致 .v5-sprite 裸 svg 以默认 300×150 显示（样式泄漏级故障）。修复=挂载+导入均在 main.tsx。LauncherSurface.tsx:487 / ProjectSurface.tsx:346 根节点自带 `v5-design` 类，`.v5-design .v5-icon` 契约直接生效，两面组件零改动。
- 三门退出码：
  - `npm run test`：**exit 0**（47 files / 570 tests 全过=基线 570 不回退）
  - `npm run build`：**exit 0**（tsc --noEmit + vite build）
  - `npm run test:render`：**exit 0**（gallery/launcher/project/loading/fidelity 原 5 报告全 PASS + 新增 APPENTRY 真实入口报告 PASS）
- 图标断言去空转化（render-smoke.mjs）：gallery/launcher/project/loading 四 fixture 面 + fidelity 两 live 面新增「sprite 图标实件（svg.v5-icon[data-graphic=icon]）计数 ≥1」与「图形对比度检查集合非空（graphics ≥1）」断言——此前「图形 ≥3:1」在零图形集合时空过（缺陷逃逸口）。新增 §11 真实入口烟测段：.v5-sprite 恰一次挂载 + 23 symbol id 全对 + sprite.css 生效（computed absolute+0×0）+ use[href^=#i-] 全可解析 + 零外部请求。
- 防回归实证（负对照）：临时摘除挂载行复跑 test:render → **exit 1**，APPENTRY 三断言拦截（spriteCount=0 / symbols=0 / css=null）；恢复后复跑 exit 0。修复前本缺陷不可被烟测拦截，修复后可拦截。去空转后各面实质首跑计数：gallery icons=221/graphics=234；launcher 10/10；project 44/56；loading 8/9；fidelity live 每视口 icons=7-8/graphics=7-11。
- 工件：`coord/runs/UI-SPRITE-MOUNT-1/`（render-smoke-appentry-report.json + screens/appentry-viewport-1440x900.png、appentry-full.png、launcher-fixture-viewport-1440x900.png、project-fixture-viewport-1440x900.png + PROGRESS.md 步骤日志）。344/345/346/347/359 原工件路径未被本卡提交改写（worktree 内冒测重写不随本卡提交）。
- 双图标体系清单（记录供后续裁定，本卡不统一）：
  1. **v5 sprite 体系**：components/icons.tsx `Icon`（svg.v5-icon + use href=#i-*；23 symbol 资产 design/sprite.tsx；渲染契约 design/sprite.css；data-graphic=icon 供烟测对比度检查）。消费方：LauncherSurface（经 ProjectBadge→Icon i-project）、ProjectSurface（Icon 直用 3 处+经宏卡/输入区组件）、LoadingCard（i-done）、TextField/MacroCard/StatusCard/ProjectBadge/VitMark 等组件、gallery。挂载=main.tsx 全局一次（本卡修复）。
  2. **lucide 内联体系**：lucide-react，自含 SVG 渲染正常。App.tsx:1-45 导入 43 枚，用于遗留 main/full/workbench surface；composer 发送/停止键 App.tsx:7096-7106（`<Send>`/`<Square>`/`<Loader2 className="spin">`）。
  3. **并存面**：发送键两体系各一份——ProjectSurface composer（v5 i-send，用户手测黑方块面=本卡修复对象）与 MainApp composer（lucide Send，无缺陷）。统一与否候主管裁定（非目标）。
- 覆盖边界（零栈卡声明）：本卡为浏览器渲染面（零栈）验证——三门全绿 + 真实入口挂载断言 + fixture 面图标实件截图在卷 + 负对照拦截实证。**真实栈表现（VitShell→agent→React 全链下发送键 i-send / 回复中 i-stop 可见性）候主管烟测/用户目检复验**；输入框 busy 态切换语义属组件行为（fixture 输入四态样张已断言），真实数据面候真栈。
- 停止条件核对：sprite.css 全局泄漏未发生——appentry 截图页面完整（无 300×150 裸块、无非图标元素样式漂移）；sprite.css 选择器全部 .v5-sprite/.v5-design 作用域，无全局裸选择器。

### 验收：待主管决策流（+用户目检发送/停止键图标可见）

### 验收（主管 2026-11-15 晚窗）

- 裁定：**pass**——diff 域栅栏亲核（仅 main.tsx +7/render-smoke.mjs +220，零越域）；三门主管复跑全绿（vitest exit 0 / build exit 0 / test:render 6 段 PASS exit 0）；**负对照主管亲证**（摘除挂载→APPENTRY_RESULT=FAIL，spriteCount=0+symbols=0 双断言拦截；main.tsx 复原后复跑全绿）。sprite.css 作用域核查接受（.v5-sprite/.v5-design 无全局裸选择器）。
- 合入：实现 `59842aa8` cherry-pick 至 main（`1ea32119`）已推；worktree wt_uisprite1 留存（362 排本卡后，装置可参考）。
- 双图标体系清单采纳在卷——统一裁定挂后续（候主管排期，非本卡域）。
- 残余：真实栈图标可见性（VitShell 全链）候 SHELL-4 烟测顺带断言或用户目检；不阻塞本卡验收。
