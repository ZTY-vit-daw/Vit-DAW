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
- 回执（2026-10-15 19:25，接管会话）：**执行完成，升级未通过硬门**——
  - 升级执行：资产 sha256 `e8f7a244…7b4117`（1,063,681,432B/1,919 文件核对）入位 vit-daw-cef-backups/（残件已清）；bin/ 487 文件 drop-in（CEF 154.0.26.0）+gdextension 换上游 v1.16.2 版（与我方 v1.15.4 版**不逐字节同构**：上游删 Windows EGL 条目=zip 内无该文件自洽，[libraries].macos 不变；diff 存档 run 目录）；替换前 v1.15.4 归档与工作树逐字节一致已核（BIN_ARCHIVE_V1154.sha256）。环境事件：zip 解包件全带 com.apple.provenance（macOS 26 对 dyld 签名校验挂起，xattr -d 被 SIP 拒）→ 整树 cp -R 重物化（487 文件 sha256 复核 bit-identical）+用户放行一次安全弹窗后通过；
  - 七项回归（全记录，`coord/runs/FE-CEF-UPGRADE-1/VALIDATION.md` @ port/fe-cef-upgrade-1=9be696bb）：**①硬门 FAIL**（OFF/ON 均 0/5——#228 焦点面修复生效：5/5 轮 synth page_hasFocus=true、卡死态不再保持、补偿无靶点；但 ⌘V 粘贴编辑命令不送达：页面 keydown×2 后零 beforeinput/input，v1.15.4 同管线立即 insertText；方向性诊断 cef_paste_diag：系统剪贴板可读/execCommand 可编辑/焦点正常=翻译层断裂）+**⑤FAIL**（页面装载 OK 但 chrome.webwebview.postMessage→CefTexture.ipc_message 零送达=webui 主链桥回归；shim ok:true，宿主脚本未动）+②PASS（Metal OSR resize FINAL pass，三档 streak=0）+⑥PASS（父进程 SIGKILL 后 helper 0.5s 全灭）+③结构在场（代理 LineEdit 在+键入 round1-3 全过+页面 composition 监听在位；真实 composition 留用户轮）+④主目录不变兼容+新增 cef-data-devtools 旁挂+⑦WINDOWS_UPGRADE_STEPS.md 回填实测值与①⑤警示；
  - 处置：停止条件（②不可绕）未触发→未回滚；前端仓 port/fe-cef-upgrade-1@**140c27cb**（GH001 未推，diff/清单存档 run 目录）；v1.15.4 仓外归档一键回滚；上游 issue 草稿 `UPSTREAM_ISSUE_DRAFT.md` 已起草**待决策侧过目**（含提交口径注意事项）；终验项在硬门修复前无意义，暂不排；
  - 端测边界声明：全部结论来自引擎边界注入驱动（push_input，页面侧 trusted）+探针页事件镜像，非 OS 键鼠模拟；自发型完整签名与真实 IME 仍需用户轮（承 FOCUSFIX 边界）；⑤的 v1.15.4 正控缺位（上轮基线被截断未达 ipc 步）已如实标注，以生产底账佐证方向。
- 验收：2026-10-15 Mac 决策侧——**升级执行与七项回归交付采信（质量过），硬门未过（①⌘V 编辑命令不送达 OFF/ON 0/5+⑤ipc 桥断）**，本卡按『升级与回归验证交付』关账；用户裁定短期无手测不回滚，前端停留 v1.16.2；断点收口+终验→FE-CEF-V1162-GAPS-1（池序 121）；上游 issue 草稿决策侧已过目，发布待用户单独批准
