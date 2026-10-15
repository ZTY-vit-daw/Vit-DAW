# FE-CEF-FOCUSFIX-1 驱动实证（补偿有效性+边界+新缺陷发现）

- 运行：2026-10-14 晚，Mac 执行侧。栈：Godot 4.6.stable（本机 Godot.app）+ 前端仓
  port/fe-cef-focusfix-1@34072f0 + 探针页=input_probe dist（vite preview 127.0.0.1:7879，
  上轮常驻服务）。日志：`autodrive_off_control.log`（补偿关）/ `autodrive_on.log`（补偿开），
  均含 VIT_CEF_DEBUG=1（[:INPROBE] 页面事件镜像）+ VIT_CEF_FOCUS_LOG=1（[VitCefFocus]
  焦点链）。

## 1. 驱动面与诚实边界（先声明）

- 输入注入=**引擎边界** `Window.push_input`（点击/键盘/⌘V；事件在页面侧 trusted=true，
  与真实键同路），**非 OS 键鼠模拟**（AGENTS §5 禁令）。点击坐标经探针页实测校准
  （gdcef OSR 转发含 device-scale 折算，本机 dpr=2：css=texture×0.5），探针浮层隐藏、
  测试组件紧凑布局后逐元素定位。
- **R1/R2 完整自发型签名（页面 blur 乒乓+焦点链反复倒手+键入全丢）无法由注入自发复现**
  ——其触发需真实 OS 焦点迁移与 IME composition 交互（R1 用户轮带中文输入法
  compositionstart/end）。本卡以**确定性合成**复现卡死态（见 §2）验证补偿机制；
  完整自发复现与终验保留 [等待用户] 轮（卡面既有项）。

## 2. 合成卡死态与补偿有效性（核心因果证据）

合成序列（驱动 `_synthesize_stuck`，模拟 R2 终态成因=插件交出浏览器焦点后迟到的
IME 事件把代理拉回 GUI 焦点而不恢复浏览器焦点，上游文档原语"a delayed IME event
cannot take it back"）：点击输入框（IME 代理就位）→ 诱饵控件 grab_focus（页面
hasFocus=false）→ 立即代理 grab_focus → 等待 3.0s 探测。

| 轮 | 补偿关（OFF）探测 | 补偿开（ON）探测 |
|---|---|---|
| 1-5（全部） | `synth page_hasFocus=false owner=.../CefTexture/@LineEdit@N`（**卡死态保持 3s+，无自主恢复**） | `synth page_hasFocus=true owner=.../CefTexture`（**自主恢复**） |

ON 轮焦点链（5 轮各一次，共 5 次 `compensation.fire`）：

```
page.focus_lost ae=native-a comp=false
compensation.fire #1 churn=5 win_focus=false page_unfocused=1279ms owner=.../@LineEdit@N
page.focus_recovered ae=native-a
compensation.after #1 win_focus=false browser_focus=true visible=true owner=.../CefTexture
```

- 检测签名按设计收敛：页面失焦 ~1.28s（阈值 1.2s）+owner 在浏览器树内+2s 窗口内
  换手 5 次（阈值 4）→ fire → 下一拍页面回报 hasFocus=true、owner 回 CefTexture
  （=host.focus 的既有可靠序列 window.grab_focus+CefTexture.grab_focus）。
- OFF 轮无任何 compensation 行（旗标关，tick 早退）——因果对照成立。
- 语义无破坏：ON 轮四步序列的常规点击/输入/焦点行为与 OFF 一致（下述 §3 的错乱
  两轮逐字节相同，补偿未加剧也未掩盖）；补偿仅在合成窗口内触发，无自持循环、
  无 backoff 触发、组合输入守卫未被触碰（注入键非 composing）。

## 3. 驱动轮发现的第二层缺陷（补偿范围外，如实上交）

四步序列×5 轮（A 输入→B 输入→回 A→B ⌘V 粘贴；每轮先合成卡死态）：两轮同为
**round 1-2 全过，round 3 起值错乱**（OFF/ON 逐字节相同，与补偿无关，放慢键入
节奏 0.09s→0.16s/键后逐字节不变=非速度伪影）。

错乱形态：round 2 的 PASTE-2 已完整入框并验证（beforeinput/input 事件齐、React
onChange 链正常）；round 3 在 B 框打入 "h" 时 DOM 值已是 `...hello2P`（被静默截断）
且迟到的 `ASTE-2` 在 "h" 后补插——`domValue: "hello1PASTE-1hello2PhASTE-2"`（探针
事件 #272，OFF 慢轮日志）。非 React 框（A·native uncontrolled）同样出现（round 4
起 abc4/xy3 错序）→ **排除 React 层**。

机制判定：⌘V 走 CEF 加速器直达页面（绕过 IME 代理文本缓冲），代理缓冲滞后；
后续焦点迁移时代理重放滞后缓冲→DOM 静默截断+迟到补插。属 gdcef IME 代理簿记层
缺陷（#236 同族），**宿主焦点补偿无法修复**（非焦点态问题），根修归上游升级线
（UPSTREAM.md §3 回归清单第 1/3 项覆盖）。

## 4. 验收结论对照卡面

| 卡面验收项 | 结果 |
|---|---|
| 补偿开：四步序列连续 5 轮全部可输入 | **部分**：键入通路全程可用（round 3-5 无一键被丢，错乱均为粘贴重放所致的**顺序**损坏，非输入失效）；严格口径（5 轮值全对）**未达**，阻断项=§3 第二层缺陷（补偿范围外） |
| 焦点链日志证明乒乓被重置 | **达成**：§2，5/5 轮补偿 fire→恢复，OFF 5/5 轮卡死保持 |
| 补偿关：原失灵行为复现（对照因果） | **达成**（合成签名面）：OFF 卡死态 3s+ 无恢复；另 round 3+ 错乱两轮同现 |
| [等待用户] 终验：产品设置页复验 | 未执行（留决策侧转用户） |

## 5. 复现命令（工件可回指）

```bash
# 探针服务（上轮常驻，若空则按 FE-CEF-KEYINPUT-RECON-1/SERVICE.md 重启）
curl -s --noproxy '*' -o /dev/null -w '%{http_code}\n' http://127.0.0.1:7879/
# OFF 对照 / ON 证明（前端仓 port/fe-cef-focusfix-1@34072f0）
cd /Users/timozty/Documents/vit-daw-frontend
VIT_CEF_DEBUG=1 VIT_CEF_FOCUS_LOG=1 VIT_CEF_AUTODRIVE_ROUNDS=5 \
  /Users/timozty/Applications/Godot.app/Contents/MacOS/Godot --path . \
  res://vit_dock/diagnostics/cef_focus_autodrive.tscn 2>&1 | tee <log>
# ON 轮加 VIT_CEF_FOCUS_COMPENSATION=1。退出码：0=全轮过，1=有败步，2=环境故障。
# 产物判定行：grep 'CEFAUTODRIVE.*(synth|verdict|FINAL)'；补偿证据：grep compensation.fire
```

- 工件：`autodrive_off_control.log` / `autodrive_on.log`（含探针镜像+焦点链全量）、
  `frontend_compensation.patch`（前端仓 commit 34072f0 全量补丁，GH001 存量二进制
  推送限制下的既定归档模式）。
- 产品语义不变性：补偿默认关（env/JSON 双通道，VitDebugFlags 热重载）；关态下
  host 仅增常量与两个 debug 访问器+一个 console 转发信号（产品链路零消费）。
