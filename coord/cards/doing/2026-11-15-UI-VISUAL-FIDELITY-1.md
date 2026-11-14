# UI-VISUAL-FIDELITY-1：launcher/工程面视觉保真返工——对照 v5.1 定稿稿

- 发卡：GLM 主管决策流 / 2026-11-15 凌晨（依据=手测 v4/v5 用户反馈：「窗口显示不全，这并不是我们设计的样式」——345/346 功能面与定稿设计稿保真度差距+窗口裁切）
- 派发确认：已确认（主管发卡，用户反馈直translate）；**与 SHELL-1 并行可跑（域不相交：webui vs 前端 C++）；SHELL-2 领取前本卡应合入**（SHELL-2 的抽屉按钮要在保真后的样式上做）
- 验收负责人：用户目检 + 主管决策流
- 池序 359；P1 / 1 天 / 依赖=344 基座+定稿稿在卷；PC；零栈（浏览器渲染烟测）

## 目标

1. **逐项对照定稿稿**（`agent/webui/design/main-window-v5/{SPEC-V5.md,REVIEW-V5.png,IMPLEMENTATION-HANDOFF.md}`+launcher 路线稿 `design/launcher-route-mondrian-v2.html` 视觉权威）：launcher surface 与 project surface 的布局/层级/留白/字级/色彩/组件密度逐项 diff，产出差距清单再修。
2. **窗口裁切修复**：用户实机「显示不全」——排查窗口尺寸策略（343 的 96%/94% 夹取 vs 用户分辨率/DPI 125%-150%），gallery 渲染烟测加多分辨率断言（1440×900+1920×1080@125% 等）。
3. 保真度修完以 `npm run test:render` 截图对照定稿稿（主管+用户目检）。

## 文件域

主仓 `agent/webui/src/launcher/**`、`src/project/**`（样式层）、`src/styles/**`、gallery 断言扩展；禁改功能语义（桥/状态机不动——纯视觉层+尺寸策略）。

## 验收标准

- 差距清单→逐项修复→`test:render` 多分辨率全 PASS+截图与定稿稿对照（用户目检 pass 为终验）。
- webui 三门不回退。

## 停止条件

- 定稿稿与实现结构冲突（如设计元素在 344 组件库缺失）→ 列缺口上交，不硬仿。

## 领取

- 时间 / origin/main hash / owner / 分支 / worktree / 领取提交：2026-11-15 晚窗 / 4685876b / freeflash（GLM-5.3 Flash）免费车道子代理 · PC · ZCode 主管会话派发 / port/ui-visual-fidelity-1 / D:/Vit_DAW_wt_uivis1 / 领取提交见本卡所在 commit
- 领取时基线：HEAD=4685876b（origin/main 最新，345/346/347/354/355 已合入）；worktree 全新检出 status 干净、diff 空
- 文件域冲突核对：doing 卡 LAUNCHER-ZEROFRICTION-1 仅取证腿（只读，已完成）、实施腿未领，无写入冲突；ARRLOOP/CLOSURE/WAVESHELL 均 Go/kernel 域不相交
- 里程碑：M0 领取→M1 差距清单（GAP-LIST.md）→M2 launcher 保真→M3 project 保真+窗口裁切→M4 多分辨率烟测；进度见 `coord/runs/UI-VISUAL-FIDELITY-1/PROGRESS.md`

## 回执/验收：待回填
