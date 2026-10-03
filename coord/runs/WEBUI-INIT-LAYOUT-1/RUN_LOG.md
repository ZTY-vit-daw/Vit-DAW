# WEBUI-INIT-LAYOUT-1 运行证据日志（PC 执行侧 flash，2026-10-03）

载体：headless 复现（静态服务 `agent/webui/dist` + playwright-core/msedge）。

## 时间线

1. **修复前 bundle（dist 构建于 18:11，未含修复）** — `repro_first_frame.mjs`
   - 1600×900@1：`shellRows="72px 391.266px 436.734px"`（3 行），workspace bottom=463.3 ≠ 900，workspace 高 391.3 与窗口高无关（三视口同为 ~423/391px 内容高）
   - 1089×578@1.75（用户截图推算视口）：workspace bottom=495.3 ≠ 578
   - 1270×674@1.5：workspace bottom=495.3 ≠ 674
   - 形态与用户截图 `coord/runs/MANUAL-TEST-20261003/webui_initial_layout_squeezed.png` 逐区吻合（侧栏 196px 媒体查询命中、右 rail 折叠、composer 悬浮中部、底部死区为 app-shell 米色底）
2. **机制验证** — `verify_mechanism_fix.mjs`（修复前 bundle，1089×578@1.75）
   - 首帧：workspace h=423.3（auto 行随内容），死区 82.7px
   - 注入对话增长内容后：auto 行撑大 → workspace bottom=578=视口底 → **"一轮对话后自行展开"的机制实证**
   - 注入修复值 `auto minmax(0,1fr)` 后：首帧即满窗（bottom=578）
3. **修复后 bundle** — `repro_first_frame.mjs` 复跑
   - 三视口 workspace/侧栏/rail/conversation-panel bottom 全部=视口底（578/674/900），`shellRows` 两行
   - `fixed_first_frame_1089x578.png`：composer 贴窗底、侧栏满高（注意：`repro_*.png` 三张被本轮复跑覆盖为修复后形态，修复前数值以本日志与 `red_oldrows_sample.json` 为准）
4. **RED 敏感性抽验** — `red_sensitivity_check.mjs`（修复后 bundle + addStyleTag 注入旧 3 行值）
   - 计算值逐字节复现修复前形态：`shellRows="72px 391.266px 436.734px"`
   - L1 断言红出两条：tracks 3≠2 children；fill slack 436.7px → 组灵敏度成立（`red_oldrows_1600x900.png`）

## 根因

`.app-shell` 声明 3 行网格（`auto auto minmax(0,1fr)`）但自 8df48982（2026-09-03 副注条整体移除）起只有 2 个子元素（TopStatusBar header + main-workspace）。空的第 3 行吃掉全部 1fr 剩余空间，main-workspace 滑入第 2 行 auto 行随内容收缩；8df48982 回执"空 auto 行 0px 无害，几何实证"实际验的是 composer 在视口内（恒真），未验不满窗。初启内容少 → 挤压；一轮对话消息增长 → auto 行撑大 → 视觉上"自行展开"。窗口越矮（用户 175% DPI，视口 ~1089×578）越显著。

修复：`agent/webui/src/styles.css` 两处 `.app-shell` grid-template-rows 改 2 行（base :60 + Mondrian 覆盖 :4129），并留注释钉死"行数=子元素数"。

## 门（本卡验收命令）

- `npm run test`：36 文件 420/420 PASS（18:26）
- `scripts/run_webui_rendered_dom_smoke.ps1`：run root `D:\Vit_DAW\artifacts\e2e_webui1\20261003_182628`，smoke_exit=0，verdict=pass，failed_groups=空；新增组 `initial-layout-L1` PASS（tracks [72px 828px]=2 children 1:1 / workspace bottom=900==innerHeight / 侧栏满高 / composer 坐底 slack 22px / 右面板贴边 right=1600）；其余既有组（IA1-IA3/msg-order/msgrevive/bucket-save-race 等）全 PASS
- HEAD=634278ff 基础上领取；实现分支 `port/webui-init-layout-1`

## 端测边界声明

headless 可完整复现初启挤压形态（与用户有头截图逐区一致），无需有头取证兜底；本卡缺陷与修复均为纯布局层，不涉及 agent 侧改动。用户目检复验（初启即展开）按卡片验收标准留给用户。
