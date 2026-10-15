# FE-CEF-UPGRADE-1：gdcef v1.15.4 → v1.16.2 升级（焦点/IME 缺陷家族根修）

- 发卡：Mac 决策侧 / 2026-10-14（依据=FE-CEF-FOCUSFIX-1 上游线结论 [UPSTREAM.md](../../runs/FE-CEF-FOCUSFIX-1/UPSTREAM.md)：#228 统一 browser/IME 焦点（macOS ARM64+Godot 4.6 烟测同型）+#236 代理编辑态修复——覆盖 KEYINPUT 取证的乒乓缺陷与 FOCUSFIX 驱动轮上交的 IME 代理文本重放缺陷两层）
- 派发确认：待用户
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 116
- 归属：Mac 线（Godot 前端宿主层）；P1 / 0.5-1 天 / 依赖=FE-CEF-FOCUSFIX-1（已 pass）
- 模型分级：L2 / GLM 执行会话

## 目标

1. **升级执行**：前端仓 addons/godot_cef/ v1.15.4 → v1.16.2 drop-in（资产 godot_cef-v1.16.2.zip ~1014MB；升级前归档现 bin/ 目录到仓外备份并记录 sha256；gdextension 同构已预核）。
2. **回归清单执行**（UPSTREAM.md §3 七项全跑，逐项记录）：
   - ①焦点面：CEF 容器四步序列（A 输入→B 输入→回 A→B ⌘V）×5 轮——**本卡核心验收**；autodrive 驱动场景复用（`res://vit_dock/diagnostics/cef_focus_autodrive.tscn`），补偿开关两态各跑（升级后缺陷应消失，OFF 轮应通过）；
   - ②accelerated OSR（Metal）渲染出画面+resize 不闪黑（#228 未在加速路径验证，必测）；
   - ③中文 IME composition 路径（compositionstart/end 事件+代理焦点行为）；
   - ④#231 per-process profile 后 cef-data 落点与既有会话兼容；
   - ⑤webui 主链：Ask Vit 打开→加载→ipc（browserBridge postMessage）→关闭重开；
   - ⑥helper watchdog 行为（父进程退出杀 helper）；
   - ⑦Windows 侧同版本替换留档（PC 线后续自行排期，本卡只记录资产与步骤）。
3. **[等待用户] 终验**：用户轮在 CEF 容器产品设置页复验原失灵序列（FE-CEF-KEYINPUT-RECON-1 底账画像：首框能输→后续键入+⌘V 失灵→按钮重置）不再失灵；含中文输入法切换场景。

## 文件域

- 前端仓 /Users/timozty/Documents/vit-daw-frontend：addons/godot_cef/（二进制替换，续用 port/fe-cef-focusfix-1 分支基础；GH001 推送限制照旧——替换前后 sha256+文件清单存档主仓 run 目录）；前端仓内零 GDScript 逻辑改动（补偿代码保留默认关，退役待升级验收后另裁）；
- 主仓：coord/runs/FE-CEF-UPGRADE-1/ 工件；agent/webui 零改动。

## 验收标准

- 七项回归全记录，焦点面①（四步×5 轮，补偿 OFF）全过为硬门；
- [等待用户] 终验通过后本卡方可 pass；升级后仍复现 macOS 加速栈特有形态 → 以双卡取证数据包（KEYINPUT ANALYSIS+FOCUSFIX VALIDATION）起草上游 issue 草稿交决策侧。

## 停止条件

- v1.16.2 与我方 Godot 4.6/加速 OSR 栈不兼容（②渲染失败且不可绕）→ 回滚 bin/ 归档，记录不兼容证据上交；
- 资产下载不可得 → 记录后暂停（不擅自换版本号）。

- 领取：2026-10-15 / origin/main ae86b218c5193772dd74f974053d304a44e8771d / Mac 执行侧（GLM 会话） / 主仓 coord-only 直推 main + 前端仓续 port/fe-cef-focusfix-1 基础（本卡新建 port/fe-cef-upgrade-1） / worktree Vit-DAW-fecef1-coord（主仓）+ 前端仓本体 / 领取提交 2a5c2101
- 接管：2026-10-15 18:28 +0800 / origin/main 9b817024ec23e539ce12f6cebd56787e09e18d47 / Mac 执行侧接管会话（GLM-5.3，ZCode；原执行流经用户裁定弃掉，本会话为接管者） / 沿用 worktree Vit-DAW-fecef1-coord（主仓）+ 前端仓 port/fe-cef-upgrade-1@34072f0（未跟踪探针 cef_resize_probe/cef_webui_chain_probe 四件续用） / 接管提交=本提交（coord-only 直推 main）。接管底账核对：bin/ 仍 v1.15.4（与仓外归档 diff -r 逐字节一致）、上轮三项基线（resize/watchdog 完整，webui_chain 跑到页面加载被截断）、资产 zip 用户已下载至桌面待校验入位
- **[等待用户:下载资产]**（2026-10-15 用户裁定：大文件下载须通知用户、由用户执行）：`godot_cef-v1.16.2.zip`（1,063,681,432 字节，https://github.com/dsh0416/godot-cef/releases/download/v1.16.2/godot_cef-v1.16.2.zip）由用户下载至 /Users/timozty/Documents/vit-daw-cef-backups/；执行侧已停止全部自行下载进程。等待期间继续不依赖资产的准备（回归场景/agent 构建/Windows 留档）。
- 回执：（commit / run ID / 七项回归记录 / 升级前后 sha256 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
