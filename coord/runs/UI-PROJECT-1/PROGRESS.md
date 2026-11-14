# UI-PROJECT-1 执行进度（freeflash 免费车道子代理，PC）

- 卡：coord/cards/doing/2026-11-14-UI-PROJECT-1.md（池序 346，P1）
- 分支：port/ui-project-1；worktree：D:/Vit_DAW_wt_uiproj1
- 基线 origin/main：45b9ed8f（依赖 344+345 已核：b05184f9/69219696/2f5cab6d ancestor OK）

## 断点恢复指南

先读卡面（doing/）+ 本文件，从最后一个未完成 M 继续。每个 M 完成即提交到 port/ui-project-1（progress-only 或混实现提交均可，回执时说明）。长命令先记命令再跑。

## 步骤日志

| # | 时间 | 动作 | 产物/退出码 | 下一步 |
|---|------|------|------------|--------|
| 1 | 2026-11-14 | fetch 复核基线 45b9ed8f、依赖三提交 ancestor OK、与 doing 卡域核对（UI-OS-1=Godot/extension/notifications，不相交） | - | worktree |
| 2 | 2026-11-14 | git worktree add D:/Vit_DAW_wt_uiproj1 -b port/ui-project-1 origin/main | HEAD=45b9ed8f | 领取提交 |
| 3 | 2026-11-14 | 领取提交：卡回填+git mv todo→doing+add，coord-only push HEAD:main | a0544d96 推 main 成功 | npm install |
| 4 | 2026-11-14 | npm install（worktree agent/webui，Playwright 钉 1.65.0 未动浏览器） | exit 0 | 勘察 |
| 5 | 2026-11-14 | 勘察：SPEC-V5/IMPLEMENTATION-HANDOFF 全文 + 344 primitives（MacroCard/StatusCard/TextField/Message/SnapshotReceipt/ActionCard 签名在卷）+ 345 LauncherSurface 分发模式 + api/launcher.ts 接缝模式 + render-smoke.mjs 断言模式 + lib/api.ts 与 types.ts 真实端点面 | 结论见 M1 提交信息 | M1 |
| 6 | 2026-11-14 | **M1 完成**：src/project/projectData.ts（线程视图模型+语义 reducer+requestMacroApply 守卫+ProjectService 接缝）+ projectData.test.ts 21 例全绿（npx vitest run src/project/projectData.test.ts，exit 0）。红线：applied 只经 macroApplySucceeded（需 pending+选择一致+非空 ref）；撤销绑定同 ref；failed 保留选择；试听不产应用；输入四态优先级 disabled>replying>focus>default；轨迹 busy/done/error；A/B preparing→ready 携带真实渲染引用 | 产物 src/project/projectData.ts(.test.ts)；commit c8a1fc1d 已推 | M2 |
| 7 | 2026-11-14 | **M2–M4 合并完成**（surface 一体成型，拆分只留在提交粒度）：ProjectSurface.tsx（上下文头/消息流/输入四态/宏卡四态/快照回执+撤销留痕/轨迹四态/AB 行）+ projectFixtures.ts（离线服务替身）+ projectService.ts（live：fetchUIState/sendChat/stopTurn，宏/A/B 无协议→live 不入口不伪造）+ project.css（v5.1 空间语言，styles.css 零改动）+ ProjectSurface.test.tsx 31 例全绿（npx vitest run src/project/ exit 0；npx tsc --noEmit exit 0） | commit 899378c2 已推 | M5 |
| 8 | 2026-11-14 | **M5 完成**：App.tsx 最小分发（type+App()+appSurface 三处，345 同款，full/main/workbench 零变化）+ gallery/main.tsx 追加 ProjectFixturePage（?surface=project，全离线 fixture）+ render-smoke.mjs 追加 §8 project 断言段（线程滚动声明 data-scroll-intentional 为声明式扫描豁免，344/345 行为不变）。**三门验收**：npm run test exit 0（43 文件 529 例）；npm run build exit 0；npm run test:render exit 0（FOUNDATION PASS + LAUNCHER PASS + PROJECT PASS）。首轮 test:render 两处 FAIL 已修：①标题栏断言改首个 root 作用域（多样例页 badge 计数误跨页）；②线程 overflow-y:auto 为设计滚动区→声明式豁免标记，状态样例框 auto 高度。工件：render-smoke-project-report.json（result=PASS，textMin 5.077:1，graphicMin 4.305:1，external=0，clip=0，tooSmall=0）+ screens/project-*.png。344/345 旧工件被复跑重生成→git restore 还原已提交版本，本卡工件只落 UI-PROJECT-1/ | commit 见下一步 | 收尾 |

## M1 架构裁定（供断点恢复者）

- 复用 344 primitives 与其类型（MacroOption/MacroCardState/TraceStep 只 import 不改）；CSS 走 src/project/project.css（域内新增），styles.css 零改动（卡面「仅新增」的子集）。
- live 数据面（零 agent 改动）：上下文头=GET /agent/ui_state（AgentUIState.project）；消息=POST /agent/chat（本 surface 独立会话，同 345 模式）；停止=POST /agent/stop_turn；轨迹=chat 请求生命周期驱动 turnStarted/turnSucceeded/turnFailed。宏快照应用/撤销/A/B 渲染在 agent 侧无现成协议 → 本卡按卡面 fixture 例外走接缝替身（fixture 显式标注），live 模式不伪造；若判定需要新协议 → 停止条件交主管。
- gallery/render-smoke 用 fixture 线程直接渲染各态（345 先例），交互型 fixture 页用 ProjectService 替身。

## 终态：DONE（执行侧自验完成，待决策验收）

执行命令与退出码（2026-11-14，worktree D:/Vit_DAW_wt_uiproj1，基线 45b9ed8f）：
- `npm run test` → exit 0（43 文件 / 529 例，含本卡新增 project 域 52 例）
- `npm run build` → exit 0（tsc --noEmit + vite build）
- `npm run test:render` → exit 0（FOUNDATION PASS / LAUNCHER PASS / PROJECT PASS）
- 领取提交 a0544d96（已推 main）；实现分支 port/ui-project-1（c8a1fc1d → 899378c2 → 16985f5f → ec30fe99 卡面回执）；协调分支 coord/ui-project-1-done = ec12c3e5（回执+doing→done，已推 main；origin/main 领取后被 349 资源登记 203c33e9 前移一次，零域交集，协调分支从新 main 重放成功）

覆盖清单（验收标准对照）：
- P3 gallery DOM：宏卡三态+（default/auditioning/applied/failed）✓、快照回执 ✓（含撤销留痕行）、A/B 按钮 ✓（idle/preparing/ready/failed）、轨迹三态 ✓（running/completed/failed + idle 行）、输入四态 ✓（default/focus/disabled/replying）
- 选择/应用分离 ✓（reducer 红线 21 例单测 + appliedWithoutSelection=0 DOM 断言）、失败/重试 ✓（原因文案 + 重试按钮 DOM 在卷）、撤销真实链路 ✓（撤销必须绑定与卡片一致的服务端 snapshotRef，单测断言 ref 不一致拒绝）
- 无伪造「已应用」✓（applied 只能经 macroApplySucceeded 事件：需 pending 在途 + 选择一致 + 非空 ref；单测三例反证）
- fixture 标注 ✓（data-fixture=notice/project-surface + 全部样例名带「（fixture）」后缀）；现有 surface 回归 ✓（529 例全绿 + FOUNDATION/LAUNCHER 渲染烟测 PASS）

端测覆盖边界声明（卡面要求）：
- 本卡验证面 = WebUI 单元测试（node 静态渲染）+ 浏览器渲染面烟测（gallery fixture 页 DOM/对比度/裁切/网络断言）。**渲染面 E2E-WEBUI-1 与旅程烟测 JOURNEY-1 两层尚未建成，本卡不得声称端到端完成**；用户旅程（工程打开→权限→实验→装载→A/B）未在本卡覆盖。
- 真实栈（AGENTS §5 三件套烟测）未执行——本卡按卡面「不占真实栈」执行；live surface（?surface=project）的真实端点消费未在真实三件套上验证，待真栈验收卡/主管裁定。
- live 模式宏快照应用/撤销/A/B 渲染无 agent 协议（service 抛错拦截面在卷）——需要协议时按停止条件交主管另开卡，本卡未改 agent。
- 左缘工程抽屉（新建/打开）不在本卡目标清单，未实现。
- App.tsx 未冲突 347（loading 事件未触碰）；A/B 开始方式保持「待用户裁定」（HANDOFF §仍待用户选择的行为），surface 只到 ready。
