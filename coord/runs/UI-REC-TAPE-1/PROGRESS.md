# UI-REC-TAPE-1 PROGRESS

- 卡：UI-REC-TAPE-1（池序 365，P1，零栈浏览器渲染面）
- worktree：D:/Vit_DAW_wt_rectape，分支 port/ui-rec-tape-1，基线 origin/main
- 开始时间：2026-11-15

## 步骤日志

- [步骤0] 会话启动：无已有 PROGRESS.md，新建本文件。任务卡/裁定/SPEC 待读。下一步=读裁定+SPEC §7.3/§7.4，核对 tapeSVG 冻结源。
- [步骤1] 勘察完成（关键结论）：
  - 冻结源 tapeSVG 在 v3/SPEC.md §3:30-51；20px 直绘时窗口宽=0（ww=bw-16<0）→ 需等比简化档（卡面已授权）；
  - 氛围系统源：launcher-route-mondrian-v2.html:474-499（PAIRS 6 色对+FNV-1a 非 ASCII 补轮+atmos L/R+rgba）；
  - 消费位：LauncherSurface.tsx:616（recents size24）/ :678（建议卡 size20）；ProjectBadge 其余消费位（ProjectSurface:360 标题栏、gallery 262/473-476/487）保留 i-project；
  - 测试面：launcherSurface.test.tsx 不断 badge（SHELL-5 域安全）；render-smoke 仅 titlebar 断 data-project-badge（不受影响）；烟测图形对比度走 [data-graphic]（stroke 优先）；
  - 收起守卫路径：entrySurfaceBridge 订阅 chrome.webview message 事件——浏览器无宿主→烟测 addInitScript 注入 mock+dispatch 真实 MessageEvent 驱动生产路径；launcher 标记锚点 data-launcher-root/drawer/recents；工程面锚点 data-loading-gate。
- [步骤2] 领取协议完成：fetch OK；worktree D:/Vit_DAW_wt_rectape @ port/ui-rec-tape-1（基线 6fb854fd）；卡 todo→doing+领取节+POOL-INDEX 365 行已提交推 main（commit e339a3ca，push exit 0）。下一步=TapeBadge 组件实现。
- [步骤3] 实现落盘：
  - components/ProjectBadge.tsx：新增 TapeBadge（≥48 冻结几何/<48 24 网格等比简化档，同拓扑）+ tapeAtmosHash/tapeAtmos/TAPE_ATMOS_PALETTE（v2 tape_atmosphere.gd 逐字移植）+ import styles/tape.css；ProjectBadge 原样保留；
  - styles/tape.css 新建（容器布局，不重绘图形）；
  - launcher/LauncherSurface.tsx 仅 3 行：import 行（ProjectBadge→TextField + 新增 TapeBadge 直引）+:616 recents 行+:678 建议卡（均传 name）；
  - design/main-window-v5/SPEC-V5.md：§7.3 v5.1.1 修订块（48px 规则不适用启动台工程列表+等比简化档定义+裁定引用）；§7.3.1 v5.1.1 场用位核对清单（6 行表格：2 启动台磁带位+4 i-project 保留位）；
  - components/tapeBadge.test.tsx 新建（10 测试：拓扑/尺寸档位/冻结令牌/哈希确定性/色对域/带量公式/氛围壳填充/中性缺省）——已跑：10/10 PASS；
  - gallery/render-smoke.mjs：头注+工件目录+launcher ldom tape 统计+6 条 tape 断言（行在场 3/3、卡 3/3、拓扑 6 枚、氛围哈希逐枚一致、带轮/窗对比度≥3:1）+磁带特写/整面截图+§12 收起守卫（真实入口+桥 mock+ack 断言+零残留断言+负对照无桥不切换）+recTape 报告聚合 allFailures。
- [步骤4] 哈希实证（node）：fixture 三名 pair 索引 0/0/2（两色分布成立）、demo vs 甲无后缀 pair 0/4——测试断言全部确定性。
- [步骤5] 三门命令预案（在 agent/webui 下执行）：
  1. npm run test（vitest run；基线 573+本卡 10=583 预期）
  2. npm run build（tsc --noEmit && vite build）
  3. npm run test:render（node src/gallery/render-smoke.mjs；工件落 coord/runs/UI-REC-TAPE-1/）

## 命令预案（长命令先记）

- git fetch origin（主仓）
- git worktree add D:/Vit_DAW_wt_rectape -b port/ui-rec-tape-1 origin/main
- cd agent/webui && npm run test（基线 573）
- npm run build
- npm run test:render
- [步骤6] 三门验证结果（2026-11-15 晚，全部 exit 0）：
  1. npm run test → 583/583 PASS（基线 573+本卡 tapeBadge.test 10）exit 0
  2. npm run build → tsc --noEmit + vite build exit 0
  3. npm run test:render → 全段 PASS exit 0；REC-TAPE_RESULT=PASS tapeBadges=6 tapeRows=3 tapeMinContrast=19.703；launcher/project/loading/fidelity/shadowbox/appentry 原断言全 PASS；代理 ECONNREFUSED(127.0.0.1:7878)=离线预期（无 agent 运行）
- [步骤7] 视觉自检 PASS：launcher-tape-badge-closeup.png 与 launcher-recents-tape-closeup.png——磁带盒（氛围壳+暖纸窗+双带轮+毂+底标线）一眼可辨，20/24px 笔触不糊，停止条件不触发；realentry-collapse-guard-after.png 证实启动台完全卸载+装载门接管（其磁带图形为 LoadingCard 既有资产）。
- [步骤8] 工件清单（coord/runs/UI-REC-TAPE-1/）：render-smoke-rec-tape-report.json + screens/{launcher-recents-tape-closeup,launcher-tape-badge-closeup,launcher-fixture-viewport-1440x900,launcher-fixture-full,realentry-collapse-guard-before,realentry-collapse-guard-after}.png。守卫关键证据：acks 含 {type:vit_surface_ack,surface:project}；launcherMarkers root:0 drawer:0 recents:0 new:0 greeting:0 thread:0；tapeBadges=0；loadingGate=true。
- 终态：DONE——本文件随后同步入 worktree 提交。
