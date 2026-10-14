# FE-CEF-KEYINPUT-RECON-1 取证结论（决策侧终判，2026-10-14）

- 数据：R1=CEF 容器探针页（user_R1_timeline_parsed.json，458 事件）；R2=CEF 容器产品设置页（user_R2_focus_chain.txt，61 行 Godot 焦点链）；R3=Safari 直开对照（用户口述全正常；执行侧 IAB 基线 browser_direct_baseline_iab_chrome146.json 同向）。
- 原始 .log（.gitignore *.log 规则）留盘于 Vit-DAW-fecef1 worktree 本 run 目录，可回指。

## 断点层终判：gdcef 插件（godot_cef CefTexture）的焦点/IME 桥乒乓

**webui/React 层清白**（三重）：焦点在场时 onChange.exec/commit/value.check 全正常；keydown 全部 trusted=true；探针页与产品页、原生框与受控框行为一致。

**页面侧证据（R1）**：每次点击输入框出现窗口级焦点乒乓 `pointerdown → window.focus → focusin(输入框) → window.blur ×2 → focusout`——页面焦点被宿主层抢走。恢复分叉：有时随后出现第二轮 `window.focus ×2 + focusin`（恢复成功→键盘/IME/粘贴可用，keydown key=Unidentified+compositionstart/end=中文输入法经 IME 通路，input 事件正常入框）；有时第二轮不来（CEF 认为浏览器整体无焦点→键入与 ⌘V 全部丢弃）。

**Godot 侧证据（R2）**：焦点链呈 `cef_texture.focus_entered → focus_exited → 焦点切至 CefTexture/@LineEdit@NNN`循环——每次页面内点击输入框，gdcef 在「浏览器纹理 ↔ 内部动态创建的隐形 LineEdit（OSR 模式 IME 桥）」间倒焦点；且第二轮起 `gui_focus_changed node=CefTexture` 不再伴随 `focus_entered`（插件焦点状态与 Godot 焦点视图脱节的直接证据），随后焦点又被 @LineEdit 抢回，无限循环。

## 用户四项观察的机制归因

| 观察 | 机制 |
|---|---|
| 第一次能输，之后失灵 | 首次点击焦点循环走完；后续乒乓停在错误端时 CEF 无焦点，输入全丢 |
| 点击按钮（标记/清空/重载/保存）"重置" | 鼠标 click 不依赖键盘焦点链，照常到达页面执行；按钮交互重启焦点循环一轮 |
| R1 探针页复现 | 缺陷与产品代码无关（容器层） |
| R3 Safari 直开正常 | 同一页面代码在真浏览器无宿主焦点层 |

## 修复面定界

- 根修=gdcef 插件层（二进制 libgdcef.dylib，仓库不可改内部）：上游 godot_cef issue/版本调研，本取证数据包可直接附 issue；
- 过渡=宿主侧焦点补偿：vit_cef_web_view_host 检测失焦态乒乓（VIT_CEF_FOCUS_LOG 已具备探测面）主动 grab_focus 重置循环——可开关 hack；
- 主仓 agent/webui 零改动。

→ 修复卡=FE-CEF-FOCUSFIX-1（池序 114；原 112/113 已被 PC 侧占用，后发者改号）。
