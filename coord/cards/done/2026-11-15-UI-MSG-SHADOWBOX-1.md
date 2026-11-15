# UI-MSG-SHADOWBOX-1：用户消息阴影框标识——两面对齐实施+SPEC-V5 修订

- 发卡：GLM 主管决策流 / 2026-11-15（依据=[用户裁定：用户消息阴影框](../../decisions/2026-11-15-user-message-shadowbox-ruling.md)）
- 派发确认：待用户触发领取；**排 UI-SPRITE-MOUNT-1（361）合入后领取**（渲染烟测断言文件域不重叠）
- 验收负责人：主管决策流 + 用户目检（用户消息来源一眼可辨）
- 池序 362；P1 / 0.25 天 / 依赖=361；PC；零栈（浏览器渲染面）

## 目标

1. **SPEC-V5 修订**：§6 对话消息「用户消息右对齐无色块」→「右对齐+阴影框标识」；S5 消息状态板补标记态；修订理由引用裁定档案。
2. **launcher surface 实施**：`.v5-launcher-msg--user`（launcher.css:227）加阴影框——复用 v5.1 空间修订「1px 灰边+柔影」令牌（窗口/宏卡同款，不新造体系），可选极浅底；`--assistant` 维持无框。
3. **project surface 对齐**：工程面用户消息同款标记（锚定 App.tsx 消息渲染+对应样式位，与 launcher 一致）。
4. **渲染断言**：test:render 断言用户消息元素带标记样式类/ computed 边框+阴影非空，两面各一；防回归。

## 非目标

- agent 消息样式变化（维持现状）。
- 磁带 icon 三态、图标体系统一（361 域/后续裁定）。

## 文件域

`agent/webui/src/styles/`（launcher.css/v5.css 或等价样式位）+ `agent/webui/src/App.tsx`（工程面消息样式类，如需）+ `agent/webui/design/main-window-v5/SPEC-V5.md` + 渲染烟测断言文件 + coord 工件；领取时回填精确清单。

## 验收标准

三门全绿（vitest/build/test:render）+两面渲染断言在卷+SPEC-V5 修订段落引用裁定档案+用户目检通过。

## 停止条件

阴影框令牌与现有消息排版冲突（行高/换行/滚动条抖动）且简改不可收敛→取证上交裁定。

## 领取/回执/验收

### 领取（2026-11-15）

- owner：GLM-5.3 Flash 执行流（ZCode 免费车道量产子代理，PC / Git Bash）
- 领取时 origin/main：2b2e3ea3f9ba46d62a9ca828977258ba3ca62bb9（含 361 合入 1ea32119 与 SHELL-4 终验收口，依赖满足；POOL-INDEX 362 已标「已解锁候领取」）
- 主仓工作树领取时状态（非本卡文件域，不触碰不整理）：main @ 2b2e3ea3；tracked 改动 `CURRENT-STATE.md`、`VitApp/Workspace/Settings/Settings.xml`、`VitApp/Workspace/default_project.xml`、`coord/decisions/2026-11-11-legame-demo-experience-first.md`、`tracktion_engine`（submodule 指针）；untracked `.vit_derived/`、`.zcode/`、design 稿等
- 执行分支：port/ui-msg-shadowbox-1（基线=origin/main 2b2e3ea3）
- worktree：D:/Vit_DAW_wt_msgsb1（零栈卡，不启真实栈，不做 PC-RUNTIME-STACK 占用登记）
- 文件域（精确）：
  - `agent/webui/src/styles/launcher.css`（.v5-launcher-msg--user 阴影框+注释）
  - `agent/webui/src/project/project.css`（.v5-project-thread .v5-message--user 同款标记+注释）——勘察结论：工程面=ProjectSurface v5 线程（App.tsx:309 渲染），样式锚点在 project.css:136，App.tsx 无需改动
  - `agent/webui/design/main-window-v5/SPEC-V5.md`（消息条款修订+裁定档案引用）
  - `agent/webui/src/gallery/render-smoke.mjs`（两面用户消息 computed 边框+阴影非空断言、--assistant/--vit 无框负断言、本卡两面截图+报告落 coord/runs/UI-MSG-SHADOWBOX-1/）
  - coord 工件（coord/runs/UI-MSG-SHADOWBOX-1/、卡面回执）
- 禁入：agent/internal/**、scripts/**、agent/webui/src/design/sprite.*、main.tsx、App.tsx、components/Message.tsx 与 components.css（344 原语文件不动，样式在 surface 层收敛）
- 勘察补充：v5.1 令牌已存在——1px 灰边=`--v5-ui-border`（#D3D3D3），柔影=`--v5-shadow-macro-card`（tokens.css:74，v5.1 宏卡同款），无需新造令牌；裁定所指「§6 对话消息」条款实际位于 SPEC-V5 §8.1 消息（§6 仅为字阶表），修订落在 §8.1

### 回执（2026-11-15，执行侧自验完成，待主管验收）

- 实现 commit：**65c659bf**（port/ui-msg-shadowbox-1，基线 2b2e3ea3；10 files +261/−9 = SPEC-V5.md + launcher.css + project.css + render-smoke.mjs + 本卡工件 6 件；分支已推 origin）
- launcher surface：`.v5-launcher-msg--user`（launcher.css:227 起）加 1px 灰边 `--v5-ui-border` + v5.1 宏卡柔影 `--v5-shadow-macro-card` + 白底 `--v5-read-bg` + 6px/10px 内边距 + `--v5-radius-control`；`--assistant` 维持无框（仅改注释）。基类 max-width 640px/line-height 1.65/overflow-wrap 不动，框含在 640px 内（`.v5-design *` box-sizing:border-box，components.css:20）。
- project surface：`.v5-project-thread .v5-message--user`（project.css:136 起）同款标记；`width: min(36em,100%)` 改 `max-width: min(36em,100%)`——原满宽在可见框下短消息出现大片空框，改后盒贴内容右收，与 launcher 视觉一致；`--vit` 无框。344 原语文件（components/Message.tsx、components.css）未动，App.tsx 未动。
- SPEC-V5 修订：§8.1 消息条款由「只靠一种手段区分（对齐方式或底色，二选一）」修订为「右对齐+阴影框标识（1px 灰边＋柔影＋极浅底，复用 v5.1 令牌）」+ S5 标记态描述（带框 vs 无框两态）+ 修订依据引用裁定档案（相对链接 ../../../../coord/decisions/…）；§11 已裁定补第 6 条（2026-11-15 用户原话+裁定档案链接）。
- 渲染断言（render-smoke.mjs，launcher §7 + project §8 两 fixture 面）：用户消息 computed 断言——borderStyle solid + borderWidth ≥1px + boxShadow 非空 + align 贴交叉轴末端 + textAlign right；`--assistant`/`--vit` 无框负断言（边框 none/0 + 阴影 none）。两面汇总报告+截图落 coord/runs/UI-MSG-SHADOWBOX-1/。
- 断言精度修正记录：首跑 FAIL 一次——本 Chromium 对 `align-self: flex-end` 的 computed 值序列化为 used value `flow-end`，断言由精确匹配 `flex-end` 放宽为 `/^(flex|flow|self)-end$/`（语义同为贴末端）后全绿；非样式缺陷。
- 三门退出码：
  - `npm run test`：**exit 0**（47 files / **573 tests** 全过=基线 573 不回退；worktree 先 `npm ci` 装依赖 108 packages）
  - `npm run build`：**exit 0**（tsc --noEmit + vite build）
  - `npm run test:render`：**exit 0**（gallery/launcher/project/loading/fidelity/appentry 全段 PASS + **SHADOWBOX_RESULT=PASS**，两面 computed frame 在卷）
- 负对照实证（防回归）：临时摘除 launcher 框样式（border:none+box-shadow:none）复跑 → LAUNCHER_RESULT=**FAIL** 拦截（borderStyle=none/0px/shadow none 三数据在卷）exit 1；复原复跑 exit 0 全绿。
- 工件：`coord/runs/UI-MSG-SHADOWBOX-1/`（PROGRESS.md 步骤日志 + render-smoke-shadowbox-report.json + screens/launcher-fixture-viewport-1440x900.png、launcher-fixture-full.png、project-fixture-viewport-1440x900.png、project-fixture-full.png）。345/346/361 等其他卡目录被冒测复跑重写的 report/截图**不随本卡提交**（留工作区，361 惯例）。
- 视觉目检（执行侧截图级）：两面截图核对——用户消息白框灰边柔影右对齐、贴内容不裁切、Vit/assistant 消息无框；clip=0、tooSmall=0、docScrollWidth 未超 1440、对比度扫描全过（launcher textMin 5.077 / graphicMin 4.305）。
- 停止条件核对：未触发——阴影框与消息排版无冲突（无裁切/无滚动抖动/无行高异常），简改即收敛。
- 观察项（留主管，非本卡域）：components/Message.tsx 头注释仍写「只靠底色区分（二选一）」（344 域旧口径），与 §8.1 新条款已不一致；ProjectSurface.test.tsx:63 用例名同旧口径（仅命名，断言内容不受影响）。是否清理由主管裁定。
- 覆盖边界（零栈卡声明）：本卡为浏览器渲染面（零栈）验证——三门全绿 + 两面 fixture computed 断言 + 负对照拦截实证 + 两面截图在卷。**真实栈表现（真数据线程下的用户消息框、窄窗/长文本换行表现、launcher live 面真实 agent 数据）候主管烟测/用户目检复验**；工程面 v5 线程仅覆盖 fixture 数据面。

### 验收：待主管决策流（+用户目检用户消息来源一眼可辨）

### 验收（主管 2026-11-15 晚窗）

- 裁定：**pass**——diff 域亲核（launcher.css/project.css/SPEC-V5/render-smoke+工件，零越域）；三门主管复跑全绿（vitest 573/573、build、test:render SHADOWBOX_RESULT=PASS）；**两面 computed 证据亲核**（launcher/project 用户消息 border solid 1px+双层柔影+右对齐，assistant 无框负断言在卷）；负对照拦截（摘框→FAIL exit 1）执行侧实证接受；工程面 width→max-width 视觉变更采纳。
- 合入：实现 `65c659bf` cherry-pick 至 main（`3a3d2973`），合并 `52641eb1`。
- 留主管观察项采纳：Message.tsx 旧口径注释/测试用例名（344 时代）下次域内卡顺手清理。
- 事故记录：执行流协调过程曾在 doing/todo 副本产生 GBK 双编码损坏（违反派发提示词的 PowerShell 禁令）；远端终态干净、损坏仅限未跟踪残留，主管已清理（doing/todo 两副本删除），零内容损失。
- 残余：真实栈表现（真数据线程/窄窗长文本/live 面）候用户目检一并复验。
