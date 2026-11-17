# 389 CONVERGE-CLEANUP-1 零栈执行回执（待主管裁定）

- owner：PC `/root/cleanup389`（Sol/high）。branch：`codex/converge-cleanup-389`。
- 唯一改动 checkout：`C:/Users/timoz/.codex/worktrees/converge-cleanup-389/Vit_DAW`。
- 领取基线：`66e951d99f10747809ef10f4f82334b1fd4d678e`；开工 status/diff 均空。依赖 388/401 的主管 PASS 与 `be43a2a2` 已核。
- 本回执表示零栈自验完成；未宣称真实栈/最终验收通过，未移动卡片。

## 改动与断言映射

1. 删除 `LauncherSurface.tsx`（git 历史保留）；gallery 基础 Section、launcher ready/loading/error/empty、launcher-live 均改用 `ConvergedLauncher`/`ConvergedThreadItem`。既有 converged 与 388 工程管理 fixture/断言完整保留。
2. 旧单测改指新壳，保留门禁、agent/内核错误、空态、loading、桥不可用、治理三键/已作答，以及 recents 可点/缺失禁用/旧 agent 缺省兼容覆盖。测试数量不减。
3. render 输入迁共享 Composer textarea，消息定位实际 `.message-row.user/.assistant .message-body > p`。右对齐验证 body text-align=right+头像第二列；保留用户边框/阴影、assistant 无框负断言。CSS 磁带卡实件替代旧 SVG badge/icon，仍断 3 recents+3 result 的壳/窗/双轮/底标、逐枚 FNV-1a 色对和带轮/窗底对比度。基础页/真实入口 sprite 挂载解析、project SVG 实件要求保留。
4. §12 真实 App 装配+仅 HTTP 数据 route fixture：切换前证新根、drawer、3 枚 data-tape-card；经生产 webview listener 分发 vit_surface=project。结果 ack=project，启动台 root/drawer/recents/new/greeting/thread 全 0，磁带卡 0，loading gate 在场；无桥不切换负对照保留。
5. ui_entry_smoke leg1 迁 UI-ENTRY-2 去回退语义：boot/health.failed/unavailable/hidden/navigate 正证，restore 零条，Godot 主窗隐藏，恰一可见 React 窗。原 autoload 开工程/同 HWND/零导航与 legacy 腿、退出码和失败日志保留。默认路径指向当前 checkout/现行前端，可显式覆盖。

未改 styles.css、App、ConvergedLauncher、ProjectSurface 或 395 文件。旧组件 import/JSX 检索 0；历史注释与 postLauncherSurfaceReady 函数名保留，不伪造文本全零。

## 命令与原始退出码

以下命令在 agent/webui，静态检查在仓库根目录。独立 npm ci，未使用 junction/共享 node_modules。

| 命令 | exit | 原始证据（本目录下 run_20261117_zerostack/） |
|---|---:|---|
| npm ci | 0 | tool stdout：170 packages in 6s；whatwg-encoding deprecation |
| npm test | 0 | vitest-raw.zip 内 vitest.log：59 files / 713 tests PASS |
| npm run build | 0 | build.log：tsc+vite PASS，既有大 chunk 提示 |
| npm run test:render attempt1 | 1 | render.log：漏迁旧 badge 截图，locator timeout（装置迁移缺陷） |
| npm run test:render attempt2 | 1 | render_attempt2.log：旧 launcher/fidelity SVG icon≥1 假设与 CSS 整卡冲突；其他段/管理/guard PASS |
| npm run test:render attempt3 | 0 | render_attempt3.log：全部 11 报告 PASS，含 ENTRY388 |
| PowerShell AST ParseFile（未执行脚本） | 0 | static-checks.log：0 parse errors |
| git diff --check | 0 | static-checks.log |
| 旧组件 import/from/JSX rg | 1（预期无匹配） | static-checks.log |

各次设置 VIT_RENDER_SMOKE_OUTPUT_ROOT 为本 run 下 render、render_attempt2、render_attempt3，未覆盖旧工件。前两次按证据修装置后才重跑，不归因环境中断或产品成功。

全部 105 份原始渲染 JSON/截图压入 render-raw.zip，CRC 与逐文件 SHA256 核对一致；archive-manifest.json 记录 archive SHA256，final-render-summary.json 可直接看 11 报告状态。自动审批拒绝删除已归档展开目录（Remove-Item，blocked by policy）；未再尝试删除，三个原始目录继续保留在本 checkout；vitest 原始输出含大量既有 VexFlow/jsdom canvas 警告，压入 vitest-raw.zip 并逐字节核对，展开 vitest.log 同样保留。本提交只纳入 ZIP、其余短日志、摘要及报告。

## 真实栈计划与覆盖边界

未启动 VitApp/agent/Godot/VitShell，未占用真实栈。render 使用 Vite+headless Chromium，生产 App/桥订阅实装配，数据/宿主桥 fixture 注入，不能替代真实宿主/内核旅程。

主管放行后，先依 PC-RUNTIME-STACK 登记/重读 owner，≤60s 净空复核；记录本分支 HEAD/status、前端 HEAD、源 fixture hash、完整命令、新 run ID。沿现有 run_entry_management_acceptance.ps1 的 Leg A/Leg B，对本 checkout 与主管指定稳定前端分别跑 vitshell_full 完整主旅程（含 failure/legacy/388 管理）和独立 ui_entry2；再运行本卡修改的 ui_entry_smoke.ps1：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/ui_entry_smoke.ps1 -RepoRoot '<389 checkout>' -GodotProjectRoot '<主管指定稳定前端>' -ProjectSource '<只读源fixture>' -RunRoot '<新独立run目录>'
```

不带 SkipBuild/SkipLegacy。仅 1 条成功路径，不宣称稳定性；同版本同确定断点两次失败停止并诊断。ui_entry 脚本已有 recents before 快照/finally 回写，但未包含哈希亲证/absent-before 清理，须沿现有外层卫生快照模式包裹：recents 与 repo/frontend Settings.xml/default_project.xml bytes/absence 前后亲核，源 fixture hash 不变，还原失败=环境中断非零退出。本卡未扩张卫生机制。

vitshell_full_smoke.ps1 及 CDP 现行 launcher/project/management 选择器与新壳对应；未修改该域，真实组合须重跑，不能引用 388 旧成功代替本卡回归。用户手测仍从 Godot 前端进入启动页，阶段出口由主管统一安排。

## 主管复核补充（零栈）

首提交 b4889ec8 后按主管复核，为同脚本 Start-GodotLeg 与 leg1 的 Godot Start-Process 加 -WindowStyle Hidden，沿用已有内核/agent/headless import 选项；只重跑 PowerShell AST 与 diff 检查，均 exit0。此选项未重跑渲染或宣称真实窗口行为验证。sprite 真实入口挂载/23 symbol/引用解析、launcher/fidelity 图形集合非空及≥3:1、文字≥4.5:1 均仍在最终 render attempt3 原始报告，未删除覆盖。
