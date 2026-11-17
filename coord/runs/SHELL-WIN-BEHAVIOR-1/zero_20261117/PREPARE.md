# 391组合装置 / 402-B 零栈准备回执

- 主仓接续`entry-final-validation`，实际基线2f45c5f1、status净；产品含主管候验389/395组合，未由执行侧改App/launcher或gallery。旧执行树不再写。
- 402-B前端独立commit1301345（从0017bba5干净领取）：3个生产gd，真实load失败/真实repo副本拒绝/共享延迟门/实际exit17，显式smoke门+direct+nonce+白名单，默认空零行为。tracked EXR M仅主管验证水合资产，未纳提交。
- 391准备0a6dfcf5已由主管merge到组合；本接续仅4个既有脚本：full ps1/CDP/wrapper/C旧入口可选guard。默认开关off保留既有行为；C只加guard不改389断言。wrapper允许本run独立kernel/agent PID且路径匹配本worktree，长import后每次spawn前重fetch/净空；无主机输入。
- `node --check scripts/vitshell_full_smoke_cdp.mjs` exit0。
- `node scripts/vitshell_full_smoke_cdp.mjs --presentation-self-test` exit0，PRESENTATION_STATIC_SELF_TEST_PASS；纯日志解析的当前nonce过滤/原始行序/错误nonce空结果，未连接browser/native/HTTP。
- PowerShell Language.Parser::ParseFile对vitshell_full_smoke.ps1、run_entry_management_acceptance.ps1、ui_entry_smoke.ps1错误数均0；git diff --check exit0。
- 主管允许安全隔离unit后执行前端`powershell -NoProfile -ExecutionPolicy Bypass -File tools/run_vitshell_readiness_unit.ps1 -RunRoot C:/Users/timoz/.codex/worktrees/godot-entry-396/vit-daw-frontend/VitShell/build/readiness-402B-zero-20261117` exit0，15/15；该minimal项目无autoload/DLL/HTTP/内核，默认分支兼容，不冒称smoke矩阵已测。raw/result附本目录。
- 安全装配unit用生产dock/ProjectRepository/eventhub+UnitProjectStore，VIT_SKIP_DEV_AUTOSTART=1、windowoff、direct0、FROM_LAUNCHER1、VSP/UDP禁用，`Godot_console --headless --path <组合前端> --script res://tools/probe_vitshell_workspace_assembly.gd --quit-after 600` exit0，7/7、无SCRIPT ERROR/Parse Error。CEF缺库与2-resource退出warning为已知环境边界，原样保留。未启VitShell/agent/kernel/HTTP；该证据只确认默认装配/新增await可编译，不替代真实注入腿。
- 最终真实矩阵预冻结normal全旅程1+delayed_success错/旧nonce配对1+scene/snapshot/timeout/exit各1，另C旧ui_entry_smoke完整独立run/exit。40ms采样首条确认在open前，有限观测不外推所有瞬态；焦点不可得报boundary/fail，不桌面点击强造。每腿完整命令、判据、预算0、stop线与早退卫生见../RUNBOOK.md。
- 当前仅装置准备；真实场景/hooks/HTTP/OS焦点/395UI负例尚未运行，需主管登记owner并正式放行。
