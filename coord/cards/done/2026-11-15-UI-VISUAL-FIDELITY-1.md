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

## 回执

- 实现 commit / 分支 / 工件分支：**port/ui-visual-fidelity-1** @ 55aca329（实现+渲染工件）+ e400e023（PROGRESS 终态 DONE）；worktree D:/Vit_DAW_wt_uivis1；领取基线 4685876b——实现留 port 待主管验收 cherry-pick 合入（工件分支=同 port 分支，§6.1）
- 差距清单：`coord/runs/UI-VISUAL-FIDELITY-1/GAP-LIST.md`（port 分支在卷）——launcher L1-L11 / project P1-P10 / 窗口裁切 W1-W2 / 多分辨率 D 节；**修复项 12**（L1 28px 屏级问候、L2 蓝壳建议动作卡 4px 角两列网格〔§5.3 禁胶囊〕、L3 抽屉工程卡 14/600+mono path、L4 消息对齐区分〔S5〕、L5 圆角纪律、L6 标题栏仅 Vit、L7 矮窄视口紧凑档、P1 用户消息 S5 右对齐无色块〔surface 层覆盖 344 原语，344 文件不动〕、P2 A/B 行平铺、P9 窗框 8px、W2 live 撑满裁切修复、D 多分辨率断言）
- **W2 裁切根因（webui 侧半边，已修）**：#root 高 100% 但 surface 根容器 auto 高 → frame--fill height:100% 退化为内容高度 → 线程变长后 composer 被推出视口且 body overflow:hidden 无滚动路径（多分辨率断言首跑实证 composer top=916>900 视口）。修复=launcher--fill/project--fill/loading-gate 高度链+launcher 线程 data-scroll-intentional+live fixture 页 100vh
- 多分辨率结果：**8/8 PASS**——1440×900@1 / 1920×1080@1 / 1536×864@1.25（Windows 125% 等价）/ 1280×720@1.5（Windows 150% 等价）× launcher-live/project-live；断言=文档无横向溢出/无未声明裁切/对比度（textMin 5.08）/最小字号/输入框+建议卡 bounding rect 在视口内/fixture 标注/零外部请求；**截图 8 张** `coord/runs/UI-VISUAL-FIDELITY-1/screens/fidelity-*.png`（供主管+用户与 p1/p3.png 目检对照）
- 三门退出码：`npm run test` **563/563 PASS exit 0**；`npm run build` **exit 0**；`npm run test:render` **五段全 PASS exit 0**（foundation/launcher/project/loading/fidelity；344-347 断言与工件路径不变，其报告因复跑顺带刷新于本分支）
- 端测覆盖边界声明：本卡=卡面注明零栈（浏览器渲染面）；**真实栈三件套烟测与用户目检未执行**——窗口裁切的宿主侧根因未动（W1），webui 侧修复的实机效果需用户手测确认；用户目检 pass 为终验
- 上交缺口（8，详见 GAP-LIST）：L8 P1 抽屉新建/打开双按钮（346 卡面已留另卡）/ L9 recent 选中态需点选语义（不虚加假态）/ P3 上下文 chip 行+引用轨道按钮（attach 语义接线）/ P4 344 MacroCard 纵排+共享轮廓 vs S5 横排三列每格轮廓（建议另开 344 基座修订卡，surface 硬掰会造第三种解读）/ P5 P3 左缘抽屉条=SHELL-2 范围（卡面已注明 SHELL-2 在保真后样式上做，本卡即前置）/ P6 344 Message 默认 canvas 气泡 vs S5 右对齐 / P7 344 TextField 发送键 icon-only vs S5 icon+文字 / P8 344 ActionCard 可撤销灰底 vs S5 蓝壳
- 移交 SHELL-1：**W1 窗口尺寸策略**——`D:/Godot/project/vit-daw-frontend/app/shell/agent_chat_window.gd` `_initial_window_size()`（L742-744）96%/94% 夹取无 DPI 逻辑/物理坐标换算、无 usable_rect（任务栏）修正、无最小尺寸下限；125-150% 缩放下 screen_get_size 与 WebView2 child hwnd 坐标基准差异=「显示不全」宿主侧根因候选（只读勘察，未改宿主）

## 验收：待主管决策验收 + 用户目检（终验）
