# 388 验收装置零栈自验

- scripts/vitshell_full_smoke.ps1新增可选-EntryManagementChecks（默认off）；cdp脚本新增management-roundtrip。不启动394/402窗口焦点验收。
- `node --check scripts/vitshell_full_smoke_cdp.mjs` exit0；`node --check agent/webui/src/gallery/render-smoke.mjs` exit0；PowerShell Language.Parser ParseFile(ps1)错误数0、exit0；`git diff --check` exit0。
- 局部真实new/open/磁带入口均要求可用并执行宿主治理cancel；settings两态真实GET；独立真实recents GET与synthetic GET503呈现分开记录；PID/绑定/HWND/current_url/launch_count+CDP主frame导航事件0+sentinel亲核，独立telemetry局部byte不变。
- stable d05043e/status无native navigate_count；不得将null当0。使用上述零导航替代证据，主管已认可。
- 补强headless新建/打开正断言：等greeting root ready后new/open enabled，再实际点new捕获new_project治理directive、点打开捕获choose_file、点磁带捕获open_project治理directive，均取消终态。第三轮render全段exit0，截图render3/CONVERGE-ENTRY-1/management-1440x900.png显示新建黑色可用。前次灰态由截图未等greeting ready造成。
- 未执行任何真实栈、未触碰用户配置。正式命令/次数/止损/卫生及稳定前端绝对路径见../RUNBOOK.md，待主管登记owner后放行。
