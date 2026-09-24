# FIX-PCA-EQCHANNEL-1 环境事件记录（§8/§9 纪律）

## 2026-09-24 栈启动与内核插件库就绪窗口

- 10:17 本地首次起栈（`stack_boot.log`；首次命令行踩 bash 反斜杠陷阱失败一次，改正斜杠后成功）。
- 10:20 pilot #1（Q10 Stereo）启动。内核插件库（Settings.xml 持久化，970+ 插件含 WaveShell）
  开机重验证窗口内：首次 `plugin_search` 返回"响应空"（懒加载触发点），旧版
  `ensure_kernel_plugin_list` 据此误判"库为空"并对 VST3 公共目录发起 `scan_plugins`，
  与后台重验证叠加 → 内核 JUCE 队列阻塞（project.state 等全部超时、内核 CPU 仅 ~90s）。
  pilot #1 于 20 分钟 settle 超时后 RuntimeError 退出（`pilot_q10_console.log`）。
- 处置：按环境中断记录；作为栈所有者强杀 VitApp/agent/Godot 全家 → 10:52 重启
  （`stack_boot2.log`，冒测全 ok）。
- 11:12 pilot #2 启动（`pilot_q10_console2.log`）；同窗口内再次触发同路径
  （代码修正前的"响应空→扫描"分支）。
- 对照证据：FIX-PCA-AUTOSWEEP-1 上轮 run_ledger 同样存在 19:26/19:46 两次
  certify phase_start（间隔 20 分钟，第二次才成功）——内核插件库开机就绪窗口
  ~20-40 分钟是该栈的既有行为，非本卡代码引入。
- 代码修正（本卡文件域内）：`ensure_kernel_plugin_list` 重写——忙（JUCE timeout）
  与空（响应且 plugin_count=0）分流；空态需连续 ~2 分钟确认才允许扫描；
  总等待窗口 20 分钟。
- 修正后策略：不再主动打扰栈，等待 pilot #2 的 settle 循环自然收敛
  （上轮先例：40 分钟内列表就绪后一切正常）。
