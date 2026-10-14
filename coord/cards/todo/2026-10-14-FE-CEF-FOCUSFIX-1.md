# FE-CEF-FOCUSFIX-1：CEF 容器输入失灵修复（上游调研+宿主焦点补偿双线）

- 发卡：Mac 决策侧 / 2026-10-14（依据=FE-CEF-KEYINPUT-RECON-1 终判：gdcef 焦点/IME 桥乒乓，[ANALYSIS.md](../../runs/FE-CEF-KEYINPUT-RECON-1/ANALYSIS.md)）
- 派发确认：已确认（用户 2026-10-14「收尾」批准取证终判+立修复卡）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 114（原 112/113 已被 PC 侧占用（rulings 51752087），后发者改号）
- 归属：Mac 线（Godot 前端宿主层）；P1 / 1-2 天 / 依赖=FE-CEF-KEYINPUT-RECON-1（已终判）
- 模型分级：L2 / GLM 执行会话

## 双线目标

1. **上游线（根修调研）**：定位 godot_cef 上游仓库——已知缺陷/issue 检索（focus/IME/LineEdit/macOS 关键词）、版本/changelog 比对（addons/godot_cef 当前版本号与上游最新）、若上游已有修复→升级路径评估（二进制替换+回归面）；若无→以本卡取证数据包（ANALYSIS.md+user_R1_timeline_parsed.json+user_R2_focus_chain.txt）起草上游 issue（发出前交决策侧过目）。
2. **宿主补偿线（过渡解）**：vit_cef_web_view_host 加可开关焦点补偿——复用 VIT_CEF_FOCUS_LOG 探测面检测乒乓失焦态（如 CefTexture 焦点被 @LineEdit 反复抢占且窗口失焦超阈值），主动 grab_focus 重置循环；默认关（env/JSON 开关），开时不破坏既有焦点语义。补偿有效性以焦点链日志前后对照证明。

## 文件域

- 前端仓（/Users/timozty/Documents/vit-daw-frontend，独立 git）：vit_dock/scenes/vit_cef_web_view_host.gd 及焦点探测相关；续用 FE-CEF-KEYINPUT-RECON-1 的分支 6e5a459 基础（其上有焦点日志+URL 钩子，推送仍受 GH001 存量二进制限制——补丁存档主仓 run 目录的既定模式）；
- 主仓：仅 coord/runs/FE-CEF-FOCUSFIX-1/ 工件；agent/webui 零改动（探针可复用 input_probe 复验）；
- 上游升级线若涉二进制替换：只评估记录路径与回归清单，不擅自替换。

## 验收标准

- 上游线：上游仓库/版本/issue 结论件（有修复=升级路径+回归清单；无修复=issue 草稿经决策侧过目）；
- 补偿线：CEF 容器内探针四步序列（A 框输入→B 框输入→回 A→B 粘贴）**连续 5 轮全部可输入**（补偿开），焦点链日志证明乒乓被重置；补偿关=原失灵行为（对照证明因果）；
- [等待用户] 终验：用户轮在产品设置页复验原失灵序列（补偿开）不再失灵。

## 停止条件

- 补偿方案加剧焦点抖动或引入新失焦形态 → 回退方案上交；
- 上游仓库不可考/插件非开源分支 → 上游线记不可行结论，补偿线为主。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 上游结论件+补偿有效性证据 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
