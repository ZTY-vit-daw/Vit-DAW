# 上游 issue 草稿：gdcef v1.16.2 macOS accelerated OSR 两处主链断裂（供决策侧过目，未提交）

- 起草：2026-10-15，Mac 执行侧（FE-CEF-UPGRADE-1 卡面验收标准触发条件："升级后仍复现
  macOS 加速栈特有形态→起草上游 issue 草稿交决策侧"）。证据包=FE-CEF-KEYINPUT-RECON-1
  ANALYSIS.md + FE-CEF-FOCUSFIX-1 VALIDATION.md + 本目录 VALIDATION.md。

## 标题（英文草拟）

Regression in v1.16.2 on macOS accelerated OSR: ⌘V paste never produces an edit
command, and window.chrome.webview.postMessage no longer reaches ipc_message

## 环境

- gdcef v1.16.2（godot_cef-v1.16.2.zip universal-apple-darwin，CEF 154.0.26.0）
- Godot 4.6.stable.official.89cea1439，macOS 26（darwin 27）ARM64（Apple M3）
- accelerated OSR = Metal（accelerated_osr_supported=true, backend=Metal）
- 宿主 GDScript 与 v1.15.4 完全一致（同一前端仓分支，仅 bin/ 替换；gdextension 用上游
  v1.16.2 版）

## 缺陷 1：⌘V 粘贴不产生编辑命令（v1.15.4 正常）

- 复现：聚焦输入框→（系统剪贴板有内容）→注入/真实 ⌘V。页面收到 KeyV keydown×2
  （key="Unidentified"，与 v1.15.4 同型），但**无 beforeinput/input/paste 任何事件**，
  DOM 值零变化。
- v1.15.4 对照（同一驱动管线）：keydown×2 后立即 `beforeinput insertText`+`input`
  （粘贴内容完整入框）。
- 已排除：系统剪贴板（宿主进程可读写，T0 pass）；DOM 编辑通路（execCommand
  insertText 正常，T2 pass）；焦点态（#228 生效——合成卡死态 5/5 轮不再保持，
  page_hasFocus=true）；页面 Clipboard API 权限（NotAllowedError 在两版一致，非路径）。
- 判定：加速键→粘贴编辑命令的翻译层在 macOS 加速 OSR 断裂（疑 #228 重排键盘/
  加速器路由后，OSR 纹理浏览器未接 macOS performKeyEquivalent→paste: 路径）。

## 缺陷 2：window.chrome.webview.postMessage 不再送达 ipc_message（v1.15.4 正常）

- 复现：页面 JS `window.chrome.webview.postMessage(obj)`（对象存在、调用无异常），
  宿主侧 `CefTexture.ipc_message` 信号零触发；v1.15.4 同宿主脚本正常闭环。
- 影响面：依赖 chrome.webview 桥的宿主应用消息主链全部断连（我方 webui Ask Vit 桥）。

## 请求

- 确认 v1.16.x macOS accelerated OSR 下 ①⌘V paste 命令 ②chrome.webview→ipc_message
  通路是否需要额外初始化/开关；或确认回归并修复。
- 若需最小工程：我可提供驱动场景（Godot 场景+探针页）与两版对照日志。

## 附：非阻塞观察（可一并反馈）

- 键入乱序残留：多轮输入后代理缓冲重放乱序（v1.15.4 已见，v1.16.2 round 4-5 仍现，
  形态变化：粘贴内容从"迟到补插"变为"整段丢失"）。
- cef-data 落点不变+新增 cef-data-devtools 旁挂（行为可接受，仅记录）。

---

### 决策侧注意（本节不进 issue）

- 提交前需用户/主管确认对外披露口径（涉及我方产品架构细节的措辞已做脱敏：桥通道、
  驱动方式=引擎边界注入）。
- 若上游先行确认缺陷 1/2 属已知问题或有 workaround（如 custom_command_line_switches
  恢复旧键盘路由），可先验证 workaround 再定提交与否。
- 缺陷 2 建议先在 PC/Windows 复测（UPSTREAM_ISSUE 草稿§环境标注 macOS；Windows 侧
  D3D12 是否同型未知，PC 线排期时一并验证）。
